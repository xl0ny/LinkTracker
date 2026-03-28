BEGIN;

DROP INDEX IF EXISTS idx_link_filter_filter_id;
DROP INDEX IF EXISTS idx_link_tag_tag_id;
DROP INDEX IF EXISTS idx_subscriptions_link_id;
DROP INDEX IF EXISTS idx_subscriptions_chat_id;

DROP TABLE IF EXISTS link_filter;
DROP TABLE IF EXISTS link_tag;
DROP TABLE IF EXISTS filter;
DROP TABLE IF EXISTS tag;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS links;
DROP TABLE IF EXISTS chats;

COMMIT;
