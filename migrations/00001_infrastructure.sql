-- +goose Up
-- This stage intentionally has no business tables. Goose still records the version.
SELECT 1;

-- +goose Down
SELECT 1;
