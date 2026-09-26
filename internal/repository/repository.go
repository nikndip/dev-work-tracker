package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dev-work-tracker/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListProjects(ctx context.Context, activeOnly bool) ([]domain.Project, error) {
	query := `SELECT id, name, hourly_rate_kopecks, is_active, created_at, updated_at FROM projects`
	if activeOnly {
		query += ` WHERE is_active = TRUE`
	}
	query += ` ORDER BY is_active DESC, lower(name), id`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var projects []domain.Project
	for rows.Next() {
		var project domain.Project
		if err := rows.Scan(&project.ID, &project.Name, &project.HourlyRateKopecks, &project.IsActive, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (r *Repository) GetProject(ctx context.Context, id int64) (domain.Project, error) {
	var project domain.Project
	err := r.pool.QueryRow(ctx, `SELECT id, name, hourly_rate_kopecks, is_active, created_at, updated_at FROM projects WHERE id=$1`, id).
		Scan(&project.ID, &project.Name, &project.HourlyRateKopecks, &project.IsActive, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return domain.Project{}, fmt.Errorf("get project: %w", err)
	}
	return project, nil
}

func (r *Repository) CreateProject(ctx context.Context, name string, rate int64) (domain.Project, error) {
	var project domain.Project
	err := r.pool.QueryRow(ctx, `INSERT INTO projects(name, hourly_rate_kopecks) VALUES($1,$2) RETURNING id,name,hourly_rate_kopecks,is_active,created_at,updated_at`, name, rate).
		Scan(&project.ID, &project.Name, &project.HourlyRateKopecks, &project.IsActive, &project.CreatedAt, &project.UpdatedAt)
	return project, wrap("create project", err)
}

func (r *Repository) UpdateProjectName(ctx context.Context, id int64, name string) (domain.Project, error) {
	return r.updateProject(ctx, `UPDATE projects SET name=$2, updated_at=now() WHERE id=$1 RETURNING id,name,hourly_rate_kopecks,is_active,created_at,updated_at`, id, name)
}

func (r *Repository) UpdateProjectRate(ctx context.Context, id, rate int64) (domain.Project, error) {
	return r.updateProject(ctx, `UPDATE projects SET hourly_rate_kopecks=$2, updated_at=now() WHERE id=$1 RETURNING id,name,hourly_rate_kopecks,is_active,created_at,updated_at`, id, rate)
}

func (r *Repository) DeactivateProject(ctx context.Context, id int64) (domain.Project, error) {
	var project domain.Project
	err := r.pool.QueryRow(ctx, `UPDATE projects SET is_active=FALSE, updated_at=now() WHERE id=$1 RETURNING id,name,hourly_rate_kopecks,is_active,created_at,updated_at`, id).
		Scan(&project.ID, &project.Name, &project.HourlyRateKopecks, &project.IsActive, &project.CreatedAt, &project.UpdatedAt)
	return project, wrap("deactivate project", err)
}

func (r *Repository) updateProject(ctx context.Context, query string, id int64, value any) (domain.Project, error) {
	var project domain.Project
	err := r.pool.QueryRow(ctx, query, id, value).
		Scan(&project.ID, &project.Name, &project.HourlyRateKopecks, &project.IsActive, &project.CreatedAt, &project.UpdatedAt)
	return project, wrap("update project", err)
}

func (r *Repository) CreateWorkEntry(ctx context.Context, entry domain.WorkEntry) (domain.WorkEntry, error) {
	var result domain.WorkEntry
	err := r.pool.QueryRow(ctx, `
		INSERT INTO work_entries(project_id,work_date,description,duration_seconds,hourly_rate_kopecks,amount_kopecks)
		VALUES($1,$2,$3,$4,$5,$6)
		RETURNING id,project_id,work_date,description,duration_seconds,hourly_rate_kopecks,amount_kopecks,created_at,updated_at`,
		entry.ProjectID, entry.WorkDate, entry.Description, entry.DurationSeconds, entry.HourlyRateKopecks, entry.AmountKopecks).
		Scan(&result.ID, &result.ProjectID, &result.WorkDate, &result.Description, &result.DurationSeconds, &result.HourlyRateKopecks, &result.AmountKopecks, &result.CreatedAt, &result.UpdatedAt)
	return result, wrap("create work entry", err)
}

const workEntrySelect = `SELECT w.id,w.project_id,p.name,w.work_date,w.description,w.duration_seconds,w.hourly_rate_kopecks,w.amount_kopecks,w.created_at,w.updated_at FROM work_entries w JOIN projects p ON p.id=w.project_id`

func (r *Repository) GetWorkEntry(ctx context.Context, id int64) (domain.WorkEntry, error) {
	return scanWorkEntry(r.pool.QueryRow(ctx, workEntrySelect+` WHERE w.id=$1`, id))
}

func (r *Repository) ListWorkEntries(ctx context.Context, start, end time.Time, projectID int64, limit, offset int) ([]domain.WorkEntry, error) {
	query := workEntrySelect + ` WHERE w.work_date >= $1 AND w.work_date < $2`
	args := []any{start, end}
	if projectID > 0 {
		query += ` AND w.project_id=$3 ORDER BY w.work_date DESC,w.id DESC LIMIT $4 OFFSET $5`
		args = append(args, projectID, limit, offset)
	} else {
		query += ` ORDER BY w.work_date DESC,w.id DESC LIMIT $3 OFFSET $4`
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list work entries: %w", err)
	}
	defer rows.Close()
	var entries []domain.WorkEntry
	for rows.Next() {
		entry, err := scanWorkEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *Repository) CountWorkEntries(ctx context.Context, start, end time.Time, projectID int64) (int, error) {
	query := `SELECT count(*) FROM work_entries WHERE work_date >= $1 AND work_date < $2`
	args := []any{start, end}
	if projectID > 0 {
		query += ` AND project_id=$3`
		args = append(args, projectID)
	}
	var count int
	err := r.pool.QueryRow(ctx, query, args...).Scan(&count)
	return count, wrap("count work entries", err)
}

func (r *Repository) UpdateWorkDescription(ctx context.Context, id int64, description string) (domain.WorkEntry, error) {
	result, err := r.pool.Exec(ctx, `UPDATE work_entries SET description=$2,updated_at=now() WHERE id=$1`, id, description)
	if err != nil {
		return domain.WorkEntry{}, fmt.Errorf("update work description: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.WorkEntry{}, pgx.ErrNoRows
	}
	return r.GetWorkEntry(ctx, id)
}

func (r *Repository) UpdateWorkDate(ctx context.Context, id int64, date time.Time) (domain.WorkEntry, error) {
	result, err := r.pool.Exec(ctx, `UPDATE work_entries SET work_date=$2,updated_at=now() WHERE id=$1`, id, date)
	if err != nil {
		return domain.WorkEntry{}, fmt.Errorf("update work date: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.WorkEntry{}, pgx.ErrNoRows
	}
	return r.GetWorkEntry(ctx, id)
}

func (r *Repository) UpdateWorkDuration(ctx context.Context, id, durationSeconds, amountKopecks int64) (domain.WorkEntry, error) {
	result, err := r.pool.Exec(ctx, `UPDATE work_entries SET duration_seconds=$2,amount_kopecks=$3,updated_at=now() WHERE id=$1`, id, durationSeconds, amountKopecks)
	if err != nil {
		return domain.WorkEntry{}, fmt.Errorf("update work duration: %w", err)
	}
	if result.RowsAffected() == 0 {
		return domain.WorkEntry{}, pgx.ErrNoRows
	}
	return r.GetWorkEntry(ctx, id)
}

func (r *Repository) DeleteWorkEntry(ctx context.Context, id int64) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM work_entries WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete work entry: %w", err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) MonthlyReport(ctx context.Context, start, end time.Time) ([]domain.ProjectReport, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT w.project_id,p.name,count(*),sum(w.duration_seconds),sum(w.amount_kopecks),
		       COALESCE(pp.status,'pending'),pp.paid_at
		FROM work_entries w
		JOIN projects p ON p.id=w.project_id
		LEFT JOIN payment_periods pp ON pp.project_id=w.project_id AND pp.period_month=$1
		WHERE w.work_date >= $1 AND w.work_date < $2
		GROUP BY w.project_id,p.name,pp.status,pp.paid_at
		ORDER BY lower(p.name),w.project_id`, start, end)
	if err != nil {
		return nil, fmt.Errorf("monthly report: %w", err)
	}
	defer rows.Close()
	var reports []domain.ProjectReport
	for rows.Next() {
		var report domain.ProjectReport
		if err := rows.Scan(&report.ProjectID, &report.ProjectName, &report.EntryCount, &report.DurationSeconds, &report.AmountKopecks, &report.PaymentStatus, &report.PaidAt); err != nil {
			return nil, fmt.Errorf("scan monthly report: %w", err)
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (r *Repository) SetPaymentStatus(ctx context.Context, projectID int64, month time.Time, status string) error {
	var paidAt any
	if status == domain.PaymentPaid {
		paidAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payment_periods(project_id,period_month,status,paid_at)
		VALUES($1,$2,$3,$4)
		ON CONFLICT(project_id,period_month) DO UPDATE
		SET status=EXCLUDED.status,paid_at=EXCLUDED.paid_at,updated_at=now()`, projectID, month, status, paidAt)
	return wrap("set payment status", err)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWorkEntry(row rowScanner) (domain.WorkEntry, error) {
	var entry domain.WorkEntry
	err := row.Scan(&entry.ID, &entry.ProjectID, &entry.ProjectName, &entry.WorkDate, &entry.Description, &entry.DurationSeconds, &entry.HourlyRateKopecks, &entry.AmountKopecks, &entry.CreatedAt, &entry.UpdatedAt)
	return entry, wrap("scan work entry", err)
}

func wrap(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", operation, pgx.ErrNoRows)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
