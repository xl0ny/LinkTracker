package pgrepo

import (
	"context"
	"fmt"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func (r *Repository) SaveOutbox(ctx context.Context, e domain.OutboxEvent) error {
	const q = `
		INSERT INTO outbox (event_id, topic, msg_key, payload)
		VALUES ($1, $2, $3, $4)`
	if _, err := r.executor(ctx).Exec(ctx, q, e.EventID, e.Topic, e.Key, e.Payload); err != nil {
		return fmt.Errorf("repo: SaveOutbox (%w)", err)
	}
	return nil
}

func (r *Repository) FetchAndLockOutbox(ctx context.Context, batchSize int, lockFor time.Duration) ([]domain.OutboxEvent, error) {
	const q = `
		UPDATE outbox
		SET available_at = now() + (interval '1 second' * $2::bigint)
		WHERE id IN (
			SELECT id FROM outbox
			WHERE status = 'pending' AND available_at <= now()
			ORDER BY id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, event_id, topic, msg_key, payload, attempts`

	rows, err := r.pool.Query(ctx, q, batchSize, int(lockFor.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("repo: FetchAndLockOutbox query (%w)", err)
	}
	defer rows.Close()

	var out []domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		if scanErr := rows.Scan(&e.ID, &e.EventID, &e.Topic, &e.Key, &e.Payload, &e.Attempts); scanErr != nil {
			return nil, fmt.Errorf("repo: FetchAndLockOutbox scan (%w)", scanErr)
		}
		out = append(out, e)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, fmt.Errorf("repo: FetchAndLockOutbox rows (%w)", rerr)
	}
	return out, nil
}

func (r *Repository) MarkOutboxSent(ctx context.Context, id int64) error {
	const q = `UPDATE outbox SET status = 'sent', sent_at = now(), last_error = NULL WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("repo: MarkOutboxSent (%w)", err)
	}
	return nil
}

func (r *Repository) IncrementOutboxAttempts(
	ctx context.Context,
	id int64,
	lastErr string,
	nextAvailableAt time.Time,
	maxAttempts int,
) error {
	const q = `
		UPDATE outbox
		SET attempts     = attempts + 1,
		    last_error   = $2,
		    available_at = $3,
		    status       = CASE WHEN attempts + 1 >= $4 THEN 'failed' ELSE 'pending' END
		WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, lastErr, nextAvailableAt, maxAttempts); err != nil {
		return fmt.Errorf("repo: IncrementOutboxAttempts (%w)", err)
	}
	return nil
}
