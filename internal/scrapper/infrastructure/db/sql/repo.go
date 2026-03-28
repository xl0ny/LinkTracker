package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, dsn string) (*repository, error) {
	pool, err := newPool(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("repo: NewRepository - pool creation error (%w)", err)
	}
	return &repository{pool: pool}, nil
}

func (r *repository) AddChat(ctx context.Context, id int64) error {
	const q = `INSERT INTO chats (telegram_id)
	VALUES ($1)
	ON CONFLICT (telegram_id) DO NOTHING`

	cmd, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("repo: AddChat - err (%w)", err)
	}

	if cmd.RowsAffected() == 0 {
		return domain.ErrChatAlreadyExists
	}

	return nil
}

func (r *repository) DeleteChat(ctx context.Context, id int64) error {
	const q = `DELETE FROM chats WHERE telegram_id = $1`

	cmd, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("repo: DeleteChat - err (%w)", err)
	}

	if cmd.RowsAffected() == 0 {
		return domain.ErrChatNotFound
	}

	return nil
}

func (r *repository) Close() {
	r.pool.Close()
}

func (r *repository) AddLink(ctx context.Context, chatID int64, link string, tags, filters *[]string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repo: AddLink - begin (%w)", err)
	}
	defer rollbackUnlessCommitted(ctx, tx)

	var internalChatID int64
	err = tx.QueryRow(ctx, `SELECT id FROM chats WHERE telegram_id = $1`, chatID).Scan(&internalChatID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.ErrChatNotFound
		}
		return fmt.Errorf("repo: AddLink - load chat (%w)", err)
	}

	var linkRowID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO links (url) VALUES ($1)
		ON CONFLICT (url) DO UPDATE SET url = links.url
		RETURNING id`, link).Scan(&linkRowID)
	if err != nil {
		return fmt.Errorf("repo: AddLink - link upsert (%w)", err)
	}

	var subID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO subscriptions (chat_id, link_id)
		VALUES ($1, $2)
		ON CONFLICT (chat_id, link_id) DO NOTHING
		RETURNING id`, internalChatID, linkRowID).Scan(&subID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.ErrLinkAlreadyExists
		}
		return fmt.Errorf("repo: AddLink - subscription (%w)", err)
	}

	tagVals := []string{}
	if tags != nil {
		tagVals = *tags
	}
	for _, v := range tagVals {
		if v == "" {
			continue
		}
		tagID, err := r.getOrCreateTagID(ctx, tx, v)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO link_tag (subscription_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, subID, tagID); err != nil {
			return fmt.Errorf("repo: AddLink - link_tag (%w)", err)
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
		filterID, err := r.getOrCreateFilterID(ctx, tx, v)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO link_filter (subscription_id, filter_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, subID, filterID); err != nil {
			return fmt.Errorf("repo: AddLink - link_filter (%w)", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repo: AddLink - commit (%w)", err)
	}
	return nil
}

func (r *repository) getOrCreateTagID(ctx context.Context, tx pgx.Tx, value string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO tag (value) VALUES ($1)
		ON CONFLICT (value) DO UPDATE SET value = tag.value
		RETURNING id`, value).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("repo: tag upsert (%w)", err)
	}
	return id, nil
}

func (r *repository) getOrCreateFilterID(ctx context.Context, tx pgx.Tx, value string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO filter (value) VALUES ($1)
		ON CONFLICT (value) DO UPDATE SET value = filter.value
		RETURNING id`, value).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("repo: filter upsert (%w)", err)
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
WHERE c.telegram_id = $1
ORDER BY s.id`
	args := []any{chatID}
	if limit > 0 {
		q += ` LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("repo: GetLinks - query (%w)", err)
	}
	defer rows.Close()

	var out []domain.Link
	for rows.Next() {
		var url string
		var lastUp *time.Time
		var tags, filters []string
		if err := rows.Scan(&url, &lastUp, &tags, &filters); err != nil {
			return nil, fmt.Errorf("repo: GetLinks - scan (%w)", err)
		}
		out = append(out, rowToDomainLink(url, lastUp, tags, filters))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: GetLinks - rows (%w)", err)
	}

	if len(out) == 0 {
		var exists int
		err := r.pool.QueryRow(ctx, `SELECT 1 FROM chats WHERE telegram_id = $1 LIMIT 1`, chatID).Scan(&exists)
		if err == pgx.ErrNoRows {
			return nil, domain.ErrChatNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("repo: GetLinks - chat check (%w)", err)
		}
	}

	return out, nil
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
	SELECT id FROM chats ORDER BY telegram_id LIMIT $1 OFFSET $2
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

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("repo: GetChats - query (%w)", err)
	}
	defer rows.Close()

	chats := make(map[int64]*domain.Chat)
	for rows.Next() {
		var tgID int64
		var url *string
		var lastUp *time.Time
		var tags, filters []string
		if err := rows.Scan(&tgID, &url, &lastUp, &tags, &filters); err != nil {
			return nil, fmt.Errorf("repo: GetChats - scan (%w)", err)
		}
		ch, ok := chats[tgID]
		if !ok {
			id := tgID
			ch = &domain.Chat{ID: &id, Links: nil}
			chats[tgID] = ch
		}
		if url != nil {
			ch.Links = append(ch.Links, rowToDomainLink(*url, lastUp, tags, filters))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: GetChats - rows (%w)", err)
	}

	out := make(map[int64]domain.Chat, len(chats))
	for id, ch := range chats {
		out[id] = *ch
	}
	return out, nil
}

func (r *repository) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("repo: DeleteLink - begin (%w)", err)
	}
	defer rollbackUnlessCommitted(ctx, tx)

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
WHERE c.telegram_id = $1 AND l.url = $2`

	var subID int64
	var url string
	var lastUp *time.Time
	var tags, filters []string
	err = tx.QueryRow(ctx, sel, chatID, linkURL).Scan(&subID, &url, &lastUp, &tags, &filters)
	if err != nil {
		if err == pgx.ErrNoRows {
			if errNF := r.ensureChatExists(ctx, tx, chatID); errNF != nil {
				return domain.Link{}, errNF
			}
			return domain.Link{}, domain.ErrLinkNotFound
		}
		return domain.Link{}, fmt.Errorf("repo: DeleteLink - select (%w)", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM subscriptions WHERE id = $1`, subID); err != nil {
		return domain.Link{}, fmt.Errorf("repo: DeleteLink - delete (%w)", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Link{}, fmt.Errorf("repo: DeleteLink - commit (%w)", err)
	}
	return rowToDomainLink(url, lastUp, tags, filters), nil
}

func (r *repository) ensureChatExists(ctx context.Context, tx pgx.Tx, telegramID int64) error {
	var one int
	err := tx.QueryRow(ctx, `SELECT 1 FROM chats WHERE telegram_id = $1 LIMIT 1`, telegramID).Scan(&one)
	if err == pgx.ErrNoRows {
		return domain.ErrChatNotFound
	}
	if err != nil {
		return fmt.Errorf("repo: ensureChatExists (%w)", err)
	}
	return nil
}

func (r *repository) UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error {
	cmdTag, err := r.pool.Exec(ctx, `
		UPDATE subscriptions s
		SET last_updated_at = $3
		FROM chats c, links l
		WHERE s.chat_id = c.id AND s.link_id = l.id
			AND c.telegram_id = $1 AND l.url = $2`, chatID, linkURL, t)
	if err != nil {
		return fmt.Errorf("repo: UpdateLinkUpdatedAt (%w)", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrLinkNotFound
	}
	return nil
}

func (r *repository) CreateTag(ctx context.Context, value string) (int64, error) {
	if value == "" {
		return 0, fmt.Errorf("repo: CreateTag - empty value")
	}
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tag (value) VALUES ($1)
		ON CONFLICT DO NOTHING
		RETURNING id`, value).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err == pgx.ErrNoRows {
		return 0, domain.ErrTagAlreadyExists
	}
	return 0, fmt.Errorf("repo: CreateTag (%w)", err)
}

func (r *repository) ListTags(ctx context.Context, limit, offset int) ([]domain.Tag, error) {
	q := `SELECT id, value FROM tag ORDER BY id`
	args := []any{}
	if limit > 0 {
		q += ` LIMIT $1 OFFSET $2`
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("repo: ListTags (%w)", err)
	}
	defer rows.Close()
	var out []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Value); err != nil {
			return nil, fmt.Errorf("repo: ListTags scan (%w)", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *repository) UpdateTag(ctx context.Context, id int64, value string) error {
	if value == "" {
		return fmt.Errorf("repo: UpdateTag - empty value")
	}
	cmd, err := r.pool.Exec(ctx, `UPDATE tag SET value = $2 WHERE id = $1`, id, value)
	if err != nil {
		return fmt.Errorf("repo: UpdateTag (%w)", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}

func (r *repository) DeleteTag(ctx context.Context, id int64) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM tag WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("repo: DeleteTag (%w)", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}

func rowToDomainLink(url string, lastUp *time.Time, tags, filters []string) domain.Link {
	if tags == nil {
		tags = []string{}
	}
	if filters == nil {
		filters = []string{}
	}
	l := domain.Link{URL: url, Tags: tags, Filters: filters}
	if lastUp != nil {
		l.LastUpdated = *lastUp
	}
	return l
}

func newPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("repo: newPool - pgx config parse err (%w)", err)
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}
