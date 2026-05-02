package pgrepo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, dsn string) (*Repository, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("repo: newPool - pgx config parse err (%w)", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("repo: NewRepository - pool creation error (%w)", err)
	}
	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) AddChat(ctx context.Context, id int64) error {
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

func (r *Repository) DeleteChat(ctx context.Context, id int64) error {
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

func (r *Repository) AddLink(ctx context.Context, chatID int64, link string, tags, filters *[]string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repo: AddLink - begin (%w)", err)
	}
	defer rollbackUnlessCommitted(ctx, tx)

	var internalChatID int64
	err = tx.QueryRow(ctx, `SELECT id FROM chats WHERE telegram_id = $1`, chatID).Scan(&internalChatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrLinkAlreadyExists
		}
		return fmt.Errorf("repo: AddLink - subscription (%w)", err)
	}

	tagVals := []string{}
	if tags != nil {
		tagVals = *tags
	}
	if insErr := r.insertLinkTags(ctx, tx, subID, tagVals); insErr != nil {
		return insErr
	}

	filterVals := []string{}
	if filters != nil {
		filterVals = *filters
	}
	if insErr := r.insertLinkFilters(ctx, tx, subID, filterVals); insErr != nil {
		return insErr
	}

	if cerr := tx.Commit(ctx); cerr != nil {
		return fmt.Errorf("repo: AddLink - commit (%w)", cerr)
	}
	return nil
}

func (r *Repository) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	const q = `
SELECT l.url, l.last_updated_at,
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
FROM (SELECT id FROM chats WHERE telegram_id = $1) c
JOIN subscriptions s ON s.chat_id = c.id
JOIN links l ON l.id = s.link_id
ORDER BY s.id`

	rows, err := r.pool.Query(ctx, q, chatID)
	if err != nil {
		return nil, fmt.Errorf("repo: GetLinks - query (%w)", err)
	}
	defer rows.Close()

	var out []domain.Link
	for rows.Next() {
		var url string
		var lastUp *time.Time
		var tags, filters []string
		if scanErr := rows.Scan(&url, &lastUp, &tags, &filters); scanErr != nil {
			return nil, fmt.Errorf("repo: GetLinks - scan (%w)", scanErr)
		}
		out = append(out, rowToDomainLink(url, lastUp, tags, filters))
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("repo: GetLinks - rows (%w)", rowsErr)
	}

	if len(out) == 0 {
		var exists int
		rowErr := r.pool.QueryRow(ctx, `SELECT 1 FROM chats WHERE telegram_id = $1 LIMIT 1`, chatID).Scan(&exists)
		if errors.Is(rowErr, pgx.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}
		if rowErr != nil {
			return nil, fmt.Errorf("repo: GetLinks - chat check (%w)", rowErr)
		}
	}

	return out, nil
}

func (r *Repository) ListSubscriptions(ctx context.Context, limit, offset int) ([]domain.Subscription, error) {
	q := `
SELECT c.telegram_id, l.url, l.last_updated_at,
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
FROM subscriptions s
JOIN chats c ON c.id = s.chat_id
JOIN links l ON l.id = s.link_id
ORDER BY s.id`
	args := []any{}
	if limit > 0 {
		q += ` LIMIT $1 OFFSET $2`
		args = append(args, limit, offset)
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("repo: ListSubscriptions - query (%w)", err)
	}
	defer rows.Close()

	var out []domain.Subscription
	for rows.Next() {
		var tgID int64
		var url string
		var lastUp *time.Time
		var tags, filters []string
		if scanErr := rows.Scan(&tgID, &url, &lastUp, &tags, &filters); scanErr != nil {
			return nil, fmt.Errorf("repo: ListSubscriptions - scan (%w)", scanErr)
		}
		out = append(out, domain.Subscription{
			ChatID: tgID,
			Link:   rowToDomainLink(url, lastUp, tags, filters),
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("repo: ListSubscriptions - rows (%w)", rowsErr)
	}
	return out, nil
}

func (r *Repository) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Link{}, fmt.Errorf("repo: DeleteLink - begin (%w)", err)
	}
	defer rollbackUnlessCommitted(ctx, tx)

	const sel = `
SELECT s.id, l.url, l.last_updated_at,
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
FROM (SELECT id FROM chats WHERE telegram_id = $1) c
JOIN subscriptions s ON s.chat_id = c.id
JOIN links l ON l.id = s.link_id AND l.url = $2`

	var subID int64
	var url string
	var lastUp *time.Time
	var tags, filters []string
	err = tx.QueryRow(ctx, sel, chatID, linkURL).Scan(&subID, &url, &lastUp, &tags, &filters)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if errNF := r.ensureChatExists(ctx, tx, chatID); errNF != nil {
				return domain.Link{}, errNF
			}
			return domain.Link{}, domain.ErrLinkNotFound
		}
		return domain.Link{}, fmt.Errorf("repo: DeleteLink - select (%w)", err)
	}

	if _, execErr := tx.Exec(ctx, `DELETE FROM subscriptions WHERE id = $1`, subID); execErr != nil {
		return domain.Link{}, fmt.Errorf("repo: DeleteLink - delete (%w)", execErr)
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return domain.Link{}, fmt.Errorf("repo: DeleteLink - commit (%w)", commitErr)
	}
	return rowToDomainLink(url, lastUp, tags, filters), nil
}

func (r *Repository) UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error {
	cmdTag, err := r.pool.Exec(ctx, `
		UPDATE links l
		SET last_updated_at = $3
		WHERE l.url = $2
			AND EXISTS (
				SELECT 1
				FROM subscriptions s
				JOIN chats c ON c.id = s.chat_id
				WHERE s.link_id = l.id AND c.telegram_id = $1
			)`, chatID, linkURL, t)
	if err != nil {
		return fmt.Errorf("repo: UpdateLinkUpdatedAt (%w)", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrLinkNotFound
	}
	return nil
}

func (r *Repository) CreateTag(ctx context.Context, value string) (int64, error) {
	if value == "" {
		return 0, errors.New("repo: CreateTag - empty value")
	}
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO tag (value) VALUES ($1)
		ON CONFLICT DO NOTHING
		RETURNING id`, value).Scan(&id)
	if err == nil {
		return id, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrTagAlreadyExists
	}
	return 0, fmt.Errorf("repo: CreateTag (%w)", err)
}

func (r *Repository) ListTags(ctx context.Context, limit, offset int) ([]domain.Tag, error) {
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
		if scanErr := rows.Scan(&t.ID, &t.Value); scanErr != nil {
			return nil, fmt.Errorf("repo: ListTags scan (%w)", scanErr)
		}
		out = append(out, t)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, fmt.Errorf("repo: ListTags rows (%w)", rerr)
	}
	return out, nil
}

func (r *Repository) UpdateTag(ctx context.Context, id int64, value string) error {
	if value == "" {
		return errors.New("repo: UpdateTag - empty value")
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

func (r *Repository) DeleteTag(ctx context.Context, id int64) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM tag WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("repo: DeleteTag (%w)", err)
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrTagNotFound
	}
	return nil
}

func (r *Repository) insertLinkTags(ctx context.Context, tx pgx.Tx, subID int64, tagVals []string) error {
	for _, v := range tagVals {
		if v == "" {
			continue
		}
		tagID, tagErr := r.getOrCreateTagID(ctx, tx, v)
		if tagErr != nil {
			return tagErr
		}
		if _, execErr := tx.Exec(ctx, `INSERT INTO link_tag (subscription_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, subID, tagID); execErr != nil {
			return fmt.Errorf("repo: AddLink - link_tag (%w)", execErr)
		}
	}
	return nil
}

func (r *Repository) insertLinkFilters(ctx context.Context, tx pgx.Tx, subID int64, filterVals []string) error {
	for _, v := range filterVals {
		if v == "" {
			continue
		}
		filterID, filterErr := r.getOrCreateFilterID(ctx, tx, v)
		if filterErr != nil {
			return filterErr
		}
		if _, execErr := tx.Exec(ctx, `INSERT INTO link_filter (subscription_id, filter_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, subID, filterID); execErr != nil {
			return fmt.Errorf("repo: AddLink - link_filter (%w)", execErr)
		}
	}
	return nil
}

func (r *Repository) getOrCreateTagID(ctx context.Context, tx pgx.Tx, value string) (int64, error) {
	var id int64
	qerr := tx.QueryRow(ctx, `
		INSERT INTO tag (value) VALUES ($1)
		ON CONFLICT (value) DO UPDATE SET value = tag.value
		RETURNING id`, value).Scan(&id)
	if qerr != nil {
		return 0, fmt.Errorf("repo: tag upsert (%w)", qerr)
	}
	return id, nil
}

func (r *Repository) getOrCreateFilterID(ctx context.Context, tx pgx.Tx, value string) (int64, error) {
	var id int64
	qerr := tx.QueryRow(ctx, `
		INSERT INTO filter (value) VALUES ($1)
		ON CONFLICT (value) DO UPDATE SET value = filter.value
		RETURNING id`, value).Scan(&id)
	if qerr != nil {
		return 0, fmt.Errorf("repo: filter upsert (%w)", qerr)
	}
	return id, nil
}

func (r *Repository) ensureChatExists(ctx context.Context, tx pgx.Tx, telegramID int64) error {
	var one int
	rowErr := tx.QueryRow(ctx, `SELECT 1 FROM chats WHERE telegram_id = $1 LIMIT 1`, telegramID).Scan(&one)
	if errors.Is(rowErr, pgx.ErrNoRows) {
		return domain.ErrChatNotFound
	}
	if rowErr != nil {
		return fmt.Errorf("repo: ensureChatExists (%w)", rowErr)
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

func rollbackUnlessCommitted(ctx context.Context, tx pgx.Tx) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Warn("pgrepo: transaction rollback", slog.String("error", err.Error()))
	}
}
