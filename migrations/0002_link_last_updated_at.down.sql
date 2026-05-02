BEGIN;

ALTER TABLE subscriptions ADD COLUMN last_updated_at TIMESTAMPTZ;

UPDATE subscriptions s
SET last_updated_at = l.last_updated_at
FROM links l
WHERE l.id = s.link_id;

ALTER TABLE links DROP COLUMN last_updated_at;

COMMIT;
