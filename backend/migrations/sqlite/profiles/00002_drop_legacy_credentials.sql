-- Credentials used to share this database. They now live in the separate
-- credentials database, so remove any copy left behind by the first schema.

-- +goose Up
DROP TABLE IF EXISTS user_credentials;

-- +goose Down
SELECT 1;
