-- +goose Up
CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE user_profiles (
    user_id        TEXT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
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
