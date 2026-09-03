ALTER TABLE rooms
    DROP CONSTRAINT rooms_created_by_fkey,
    ADD CONSTRAINT rooms_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE messages
    DROP CONSTRAINT messages_user_id_fkey,
    ADD CONSTRAINT messages_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
