ALTER TABLE messages DROP CONSTRAINT messages_retention;

DELETE FROM messages
WHERE created_at + INTERVAL '30 minutes' <= NOW();

UPDATE messages
SET expires_at = created_at + INTERVAL '30 minutes';

ALTER TABLE messages
    ADD CONSTRAINT messages_retention CHECK (expires_at = created_at + INTERVAL '30 minutes');
