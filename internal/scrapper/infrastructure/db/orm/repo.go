package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/orm/model"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/orm/query"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(ctx context.Context, dsn string) (*repository, error) {
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("orm repo: open (%w)", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("orm repo: sql db (%w)", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		if closeErr := sqlDB.Close(); closeErr != nil {
			slog.Warn("orm repo: close after failed ping", slog.String("error", closeErr.Error()))
		}
		return nil, fmt.Errorf("orm repo: ping (%w)", err)
	}
	return &repository{db: gdb}, nil
}

func (r *repository) Close() {
	sqlDB, err := r.db.DB()
	if err != nil {
		slog.Warn("orm repo: underlying sql db", slog.String("error", err.Error()))
		return
	}
	if err := sqlDB.Close(); err != nil {
		slog.Warn("orm repo: close", slog.String("error", err.Error()))
	}
}

func (r *repository) AddChat(ctx context.Context, id int64) error {
	c := model.Chat{TelegramID: id}
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&c)
	if res.Error != nil {
		return fmt.Errorf("orm repo: AddChat (%w)", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrChatAlreadyExists
	}
	return nil
}

func (r *repository) DeleteChat(ctx context.Context, id int64) error {
	q := query.Use(r.db)
	info, err := q.Chat.WithContext(ctx).Where(q.Chat.TelegramID.Eq(id)).Delete()
	if err != nil {
		return fmt.Errorf("orm repo: DeleteChat (%w)", err)
	}
	if info.RowsAffected == 0 {
		return domain.ErrChatNotFound
	}
	return nil
}

func (r *repository) AddLink(ctx context.Context, chatID int64, link string, tags, filters *[]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var internalChatID int64
		row := tx.Raw(`SELECT id FROM chats WHERE telegram_id = ?`, chatID).Row()
		if err := row.Scan(&internalChatID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domain.ErrChatNotFound
			}
			return fmt.Errorf("orm repo: AddLink load chat (%w)", err)
		}

		var linkRowID int64
		if err := tx.Raw(`
			INSERT INTO links (url) VALUES (?)
			ON CONFLICT (url) DO UPDATE SET url = links.url
			RETURNING id`, link).Scan(&linkRowID).Error; err != nil {
			return fmt.Errorf("orm repo: AddLink link upsert (%w)", err)
		}

		var subID int64
		if err := tx.Raw(`
			INSERT INTO subscriptions (chat_id, link_id)
			VALUES (?, ?)
			ON CONFLICT (chat_id, link_id) DO NOTHING
			RETURNING id`, internalChatID, linkRowID).Scan(&subID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
				return domain.ErrLinkAlreadyExists
			}
			return fmt.Errorf("orm repo: AddLink subscription (%w)", err)
		}
		if subID == 0 {
			return domain.ErrLinkAlreadyExists
		}

		tagVals := []string{}
		if tags != nil {
			tagVals = *tags
		}
		for _, v := range tagVals {
			if v == "" {
				continue
			}
			tagID, err := getOrCreateTagID(tx, v)
			if err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO link_tag (subscription_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, subID, tagID).Error; err != nil {
				return fmt.Errorf("orm repo: AddLink link_tag (%w)", err)
			}
		}

		filterVals := []string{}
		if filters != nil {
			filterVals = *filters
		}
		for _, v := range filterVals {
			if v == "" {
				continue
			}
			filterID, err := getOrCreateFilterID(tx, v)
			if err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO link_filter (subscription_id, filter_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, subID, filterID).Error; err != nil {
				return fmt.Errorf("orm repo: AddLink link_filter (%w)", err)
			}
		}
		return nil
	})
}

func getOrCreateTagID(tx *gorm.DB, value string) (int64, error) {
	var id int64
	if err := tx.Raw(`
		INSERT INTO tag (value) VALUES (?)
		ON CONFLICT (value) DO UPDATE SET value = tag.value
		RETURNING id`, value).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("orm repo: tag upsert (%w)", err)
	}
	return id, nil
}

func getOrCreateFilterID(tx *gorm.DB, value string) (int64, error) {
	var id int64
	if err := tx.Raw(`
		INSERT INTO filter (value) VALUES (?)
		ON CONFLICT (value) DO UPDATE SET value = filter.value
		RETURNING id`, value).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("orm repo: filter upsert (%w)", err)
	}
	return id, nil
}

func (r *repository) GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	q := `
SELECT l.url, s.last_updated_at,
	COALESCE((
		SELECT array_agg(t.value ORDER BY t.value)
		FROM link_tag lt JOIN tag t ON t.id = lt.tag_id
		WHERE lt.subscription_id = s.id
	), ARRAY[]::text[]),
	COALESCE((
		SELECT array_agg(f.value ORDER BY f.value)
		FROM link_filter lf JOIN filter f ON f.id = lf.filter_id
		WHERE lf.subscription_id = s.id
	), ARRAY[]::text[])
FROM chats c
JOIN subscriptions s ON s.chat_id = c.id
JOIN links l ON l.id = s.link_id
WHERE c.telegram_id = ?
ORDER BY s.id`
	args := []any{chatID}
	if limit > 0 {
		q += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	rows, err := r.db.WithContext(ctx).Raw(q, args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("orm repo: GetLinks query (%w)", err)
	}
	defer closeSQLRows(rows, "GetLinks")

	var out []domain.Link
	for rows.Next() {
		lk, err := scanLinkRow(rows)
		if err != nil {
			return nil, fmt.Errorf("orm repo: GetLinks scan (%w)", err)
		}
		out = append(out, lk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orm repo: GetLinks rows (%w)", err)
	}

	if len(out) == 0 {
		var exists int
		row := r.db.WithContext(ctx).Raw(`SELECT 1 FROM chats WHERE telegram_id = ? LIMIT 1`, chatID).Row()
		if err := row.Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, domain.ErrChatNotFound
			}
			return nil, fmt.Errorf("orm repo: GetLinks chat check (%w)", err)
		}
	}

	return out, nil
}

func scanLinkRow(rows *sql.Rows) (domain.Link, error) {
	var url string
	var lastUp sql.NullTime
	var tags, filters pq.StringArray
	if err := rows.Scan(&url, &lastUp, &tags, &filters); err != nil {
		return domain.Link{}, err
	}
	l := domain.Link{URL: url, Tags: []string(tags), Filters: []string(filters)}
	if lastUp.Valid {
		l.LastUpdated = lastUp.Time
	}
	return l, nil
}

func (r *repository) GetChats(ctx context.Context, limit, offset int) (map[int64]domain.Chat, error) {
	q := `
SELECT c.telegram_id, l.url, s.last_updated_at,
	COALESCE((
		SELECT array_agg(t.value ORDER BY t.value)
		FROM link_tag lt JOIN tag t ON t.id = lt.tag_id
		WHERE lt.subscription_id = s.id
	), ARRAY[]::text[]),
	COALESCE((
		SELECT array_agg(f.value ORDER BY f.value)
		FROM link_filter lf JOIN filter f ON f.id = lf.filter_id
		WHERE lf.subscription_id = s.id
	), ARRAY[]::text[])
FROM chats c
LEFT JOIN subscriptions s ON s.chat_id = c.id
LEFT JOIN links l ON l.id = s.link_id AND s.id IS NOT NULL`
	args := []any{}
	if limit > 0 {
		q = `
WITH paged AS (
	SELECT id FROM chats ORDER BY telegram_id LIMIT ? OFFSET ?
)
SELECT c.telegram_id, l.url, s.last_updated_at,
	COALESCE((
		SELECT array_agg(t.value ORDER BY t.value)
		FROM link_tag lt JOIN tag t ON t.id = lt.tag_id
		WHERE lt.subscription_id = s.id
	), ARRAY[]::text[]),
	COALESCE((
		SELECT array_agg(f.value ORDER BY f.value)
		FROM link_filter lf JOIN filter f ON f.id = lf.filter_id
		WHERE lf.subscription_id = s.id
	), ARRAY[]::text[])
FROM paged p
JOIN chats c ON c.id = p.id
LEFT JOIN subscriptions s ON s.chat_id = c.id
LEFT JOIN links l ON l.id = s.link_id AND s.id IS NOT NULL`
		args = append(args, limit, offset)
	}
	q += `
ORDER BY c.telegram_id, s.id`

	rows, err := r.db.WithContext(ctx).Raw(q, args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("orm repo: GetChats query (%w)", err)
	}
	defer closeSQLRows(rows, "GetChats")

	chats := make(map[int64]*domain.Chat)
	for rows.Next() {
		var tgID int64
		var url *string
		var lastUp sql.NullTime
		var tags, filters pq.StringArray
		if err := rows.Scan(&tgID, &url, &lastUp, &tags, &filters); err != nil {
			return nil, fmt.Errorf("orm repo: GetChats scan (%w)", err)
		}
		ch, ok := chats[tgID]
		if !ok {
			id := tgID
			ch = &domain.Chat{ID: &id, Links: nil}
			chats[tgID] = ch
		}
		if url != nil {
			l := domain.Link{URL: *url, Tags: []string(tags), Filters: []string(filters)}
			if lastUp.Valid {
				l.LastUpdated = lastUp.Time
			}
			ch.Links = append(ch.Links, l)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orm repo: GetChats rows (%w)", err)
	}

	out := make(map[int64]domain.Chat, len(chats))
	for id, ch := range chats {
		out[id] = *ch
	}
	return out, nil
}

func (r *repository) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	var out domain.Link
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		const sel = `
SELECT s.id, l.url, s.last_updated_at,
	COALESCE((
		SELECT array_agg(t.value ORDER BY t.value)
		FROM link_tag lt JOIN tag t ON t.id = lt.tag_id
		WHERE lt.subscription_id = s.id
	), ARRAY[]::text[]),
	COALESCE((
		SELECT array_agg(f.value ORDER BY f.value)
		FROM link_filter lf JOIN filter f ON f.id = lf.filter_id
		WHERE lf.subscription_id = s.id
	), ARRAY[]::text[])
FROM chats c
JOIN subscriptions s ON s.chat_id = c.id
JOIN links l ON l.id = s.link_id
WHERE c.telegram_id = ? AND l.url = ?`

		rows, err := tx.Raw(sel, chatID, linkURL).Rows()
		if err != nil {
			return fmt.Errorf("orm repo: DeleteLink select (%w)", err)
		}
		if !rows.Next() {
			closeSQLRows(rows, "DeleteLink select (no row)")
			if err := ensureChatExists(tx, chatID); err != nil {
				return err
			}
			return domain.ErrLinkNotFound
		}
		var subID int64
		var url string
		var lastUp sql.NullTime
		var tags, filters pq.StringArray
		if err := rows.Scan(&subID, &url, &lastUp, &tags, &filters); err != nil {
			closeSQLRows(rows, "DeleteLink select (scan)")
			return fmt.Errorf("orm repo: DeleteLink scan (%w)", err)
		}
		out = domain.Link{URL: url, Tags: []string(tags), Filters: []string(filters)}
		if lastUp.Valid {
			out.LastUpdated = lastUp.Time
		}
		closeSQLRows(rows, "DeleteLink select")
		if err := tx.Exec(`DELETE FROM subscriptions WHERE id = ?`, subID).Error; err != nil {
			return fmt.Errorf("orm repo: DeleteLink delete (%w)", err)
		}
		return nil
	})
	if err != nil {
		return domain.Link{}, err
	}
	return out, nil
}

func ensureChatExists(tx *gorm.DB, telegramID int64) error {
	var one int
	row := tx.Raw(`SELECT 1 FROM chats WHERE telegram_id = ? LIMIT 1`, telegramID).Row()
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrChatNotFound
		}
		return fmt.Errorf("orm repo: ensureChatExists (%w)", err)
	}
	return nil
}

func (r *repository) UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE subscriptions s
		SET last_updated_at = ?
		FROM chats c, links l
		WHERE s.chat_id = c.id AND s.link_id = l.id
			AND c.telegram_id = ? AND l.url = ?`, t, chatID, linkURL)
	if res.Error != nil {
		return fmt.Errorf("orm repo: UpdateLinkUpdatedAt (%w)", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrLinkNotFound
	}
	return nil
}

func (r *repository) CreateTag(ctx context.Context, value string) (int64, error) {
	if value == "" {
		return 0, fmt.Errorf("orm repo: CreateTag - empty value")
	}
	q := query.Use(r.db)
	row := &model.Tag{Value: value}
	if err := q.Tag.WithContext(ctx).Create(row); err != nil {
		if isPGUniqueViolation(err) {
			return 0, domain.ErrTagAlreadyExists
		}
		return 0, fmt.Errorf("orm repo: CreateTag (%w)", err)
	}
	return row.ID, nil
}

func (r *repository) ListTags(ctx context.Context, limit, offset int) ([]domain.Tag, error) {
	q := query.Use(r.db)
	b := q.Tag.WithContext(ctx).Order(q.Tag.ID)
	if limit > 0 {
		b = b.Limit(limit).Offset(offset)
	}
	items, err := b.Find()
	if err != nil {
		return nil, fmt.Errorf("orm repo: ListTags (%w)", err)
	}
	out := make([]domain.Tag, 0, len(items))
	for _, it := range items {
		out = append(out, domain.Tag{ID: it.ID, Value: it.Value})
	}
	return out, nil
}

func (r *repository) UpdateTag(ctx context.Context, id int64, value string) error {
	if value == "" {
		return fmt.Errorf("orm repo: UpdateTag - empty value")
	}
	q := query.Use(r.db)
	info, err := q.Tag.WithContext(ctx).Where(q.Tag.ID.Eq(id)).Update(q.Tag.Value, value)
	if err != nil {
		if isPGUniqueViolation(err) {
			return domain.ErrTagAlreadyExists
		}
		return fmt.Errorf("orm repo: UpdateTag (%w)", err)
	}
	if info.RowsAffected == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}

func (r *repository) DeleteTag(ctx context.Context, id int64) error {
	q := query.Use(r.db)
	info, err := q.Tag.WithContext(ctx).Where(q.Tag.ID.Eq(id)).Delete()
	if err != nil {
		return fmt.Errorf("orm repo: DeleteTag (%w)", err)
	}
	if info.RowsAffected == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}
