-- +goose Up
CREATE TABLE projects (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(trim(name)) BETWEEN 1 AND 200),
    hourly_rate_kopecks BIGINT NOT NULL CHECK (hourly_rate_kopecks > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE work_entries (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id),
    work_date DATE NOT NULL,
    description TEXT NOT NULL CHECK (char_length(trim(description)) BETWEEN 1 AND 4000),
    duration_seconds BIGINT NOT NULL CHECK (duration_seconds > 0 AND duration_seconds <= 86400),
    hourly_rate_kopecks BIGINT NOT NULL CHECK (hourly_rate_kopecks > 0),
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX work_entries_project_id_idx ON work_entries(project_id);
CREATE INDEX work_entries_work_date_idx ON work_entries(work_date);
CREATE INDEX work_entries_project_date_idx ON work_entries(project_id, work_date);

CREATE TABLE payment_periods (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id),
    period_month DATE NOT NULL CHECK (EXTRACT(DAY FROM period_month) = 1),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid')),
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, period_month)
);

CREATE INDEX payment_periods_month_idx ON payment_periods(period_month);

-- +goose Down
DROP TABLE IF EXISTS payment_periods;
DROP TABLE IF EXISTS work_entries;
DROP TABLE IF EXISTS projects;
