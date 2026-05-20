BEGIN;

ALTER TABLE links ADD COLUMN last_updated_at TIMESTAMPTZ;

UPDATE links l
SET last_updated_at = sub.max_lu
FROM (
    SELECT link_id, MAX(last_updated_at) AS max_lu
    FROM subscriptions
    WHERE last_updated_at IS NOT NULL
    GROUP BY link_id
) sub
WHERE sub.link_id = l.id;

ALTER TABLE subscriptions DROP COLUMN last_updated_at;

COMMIT;
