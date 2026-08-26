CREATE TABLE users (
    id UUID PRIMARY KEY,
    username VARCHAR(24) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ,
    CONSTRAINT users_username_format CHECK (username ~ '^[a-z0-9_]{3,24}$')
);

CREATE TABLE rooms (
    id UUID PRIMARY KEY,
    name VARCHAR(32) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT rooms_name_format CHECK (name ~ '^[a-z0-9_-]{3,32}$')
);

CREATE TABLE messages (
    id UUID PRIMARY KEY,
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    content VARCHAR(2000) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT messages_content_length CHECK (char_length(content) BETWEEN 1 AND 2000),
    CONSTRAINT messages_retention CHECK (expires_at = created_at + INTERVAL '7 days')
);

CREATE INDEX messages_room_created_idx ON messages (room_id, created_at DESC, id DESC);
CREATE INDEX messages_expires_at_idx ON messages (expires_at);
