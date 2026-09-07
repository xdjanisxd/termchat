ALTER TABLE messages DROP CONSTRAINT messages_retention;

UPDATE messages
SET expires_at = created_at + INTERVAL '7 days';

ALTER TABLE messages
    ADD CONSTRAINT messages_retention CHECK (expires_at = created_at + INTERVAL '7 days');
