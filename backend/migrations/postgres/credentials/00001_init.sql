-- user_id refers to users.id in the profiles database. There is no foreign key
-- because the two live in separate databases; the repository keeps them
-- consistent.

-- +goose Up
CREATE TABLE user_credentials (
    id           UUID PRIMARY KEY,
    user_id      UUID NOT NULL,
    method       TEXT NOT NULL CHECK (method IN ('password', 'oauth', 'passkey')),
    username     TEXT NOT NULL,
    secret_hash  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    UNIQUE (method, username)
);

CREATE INDEX idx_credentials_user_id ON user_credentials (user_id);
CREATE INDEX idx_credentials_username_lower ON user_credentials (lower(username));

-- +goose Down
DROP TABLE user_credentials;
