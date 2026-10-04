-- +goose Up
CREATE TABLE users (
    id         UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_profiles (
    user_id        UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    phone          TEXT NOT NULL,
    street_address TEXT NOT NULL DEFAULT '',
    locality       TEXT NOT NULL DEFAULT '',
    region         TEXT NOT NULL DEFAULT '',
    postal_code    TEXT NOT NULL DEFAULT '',
    country        TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_profiles_phone ON user_profiles (phone);
CREATE INDEX idx_profiles_name_lower ON user_profiles (lower(name));

-- +goose Down
DROP TABLE user_profiles;
DROP TABLE users;
