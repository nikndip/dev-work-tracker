package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"dev-work-tracker/internal/domain"
	"dev-work-tracker/internal/money"
)

const (
	MaxProjectNameLength = 200
	MaxDescriptionLength = 4000
)

type Repository interface {
	ListProjects(context.Context, bool) ([]domain.Project, error)
	GetProject(context.Context, int64) (domain.Project, error)
	CreateProject(context.Context, string, int64) (domain.Project, error)
	UpdateProjectName(context.Context, int64, string) (domain.Project, error)
	UpdateProjectRate(context.Context, int64, int64) (domain.Project, error)
	DeactivateProject(context.Context, int64) (domain.Project, error)
	CreateWorkEntry(context.Context, domain.WorkEntry) (domain.WorkEntry, error)
	GetWorkEntry(context.Context, int64) (domain.WorkEntry, error)
	ListWorkEntries(context.Context, time.Time, time.Time, int64, int, int) ([]domain.WorkEntry, error)
	CountWorkEntries(context.Context, time.Time, time.Time, int64) (int, error)
	UpdateWorkDescription(context.Context, int64, string) (domain.WorkEntry, error)
	UpdateWorkDate(context.Context, int64, time.Time) (domain.WorkEntry, error)
	UpdateWorkDuration(context.Context, int64, int64, int64) (domain.WorkEntry, error)
	DeleteWorkEntry(context.Context, int64) error
	MonthlyReport(context.Context, time.Time, time.Time) ([]domain.ProjectReport, error)
	SetPaymentStatus(context.Context, int64, time.Time, string) error
}

type Service struct {
	repo        Repository
	defaultRate int64
	location    *time.Location
	now         func() time.Time
}

func New(repo Repository, defaultRate int64, location *time.Location) *Service {
	return &Service{repo: repo, defaultRate: defaultRate, location: location, now: time.Now}
}

func (s *Service) Location() *time.Location { return s.location }
func (s *Service) DefaultRate() int64       { return s.defaultRate }

func (s *Service) ListProjects(ctx context.Context, activeOnly bool) ([]domain.Project, error) {
	return s.repo.ListProjects(ctx, activeOnly)
}

func (s *Service) GetProject(ctx context.Context, id int64) (domain.Project, error) {
	return s.repo.GetProject(ctx, id)
}

func (s *Service) CreateProject(ctx context.Context, name string, rate int64) (domain.Project, error) {
	name, err := validateText(name, MaxProjectNameLength, "название проекта")
	if err != nil {
		return domain.Project{}, err
	}
	if rate <= 0 {
		return domain.Project{}, errors.New("ставка должна быть положительной")
	}
	return s.repo.CreateProject(ctx, name, rate)
}

func (s *Service) UpdateProjectName(ctx context.Context, id int64, name string) (domain.Project, error) {
	name, err := validateText(name, MaxProjectNameLength, "название проекта")
	if err != nil {
		return domain.Project{}, err
	}
	return s.repo.UpdateProjectName(ctx, id, name)
}

func (s *Service) UpdateProjectRate(ctx context.Context, id, rate int64) (domain.Project, error) {
	if rate <= 0 {
		return domain.Project{}, errors.New("ставка должна быть положительной")
	}
	return s.repo.UpdateProjectRate(ctx, id, rate)
}

func (s *Service) DeactivateProject(ctx context.Context, id int64) (domain.Project, error) {
	return s.repo.DeactivateProject(ctx, id)
}

func (s *Service) CreateWorkEntry(ctx context.Context, projectID int64, date time.Time, description string, durationSeconds int64) (domain.WorkEntry, error) {
	project, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return domain.WorkEntry{}, err
	}
	if !project.IsActive {
		return domain.WorkEntry{}, errors.New("проект деактивирован")
	}
	description, err = validateText(description, MaxDescriptionLength, "описание")
	if err != nil {
		return domain.WorkEntry{}, err
	}
	date, err = s.ValidateWorkDate(date)
	if err != nil {
		return domain.WorkEntry{}, err
	}
	amount, err := money.CalculateAmount(project.HourlyRateKopecks, durationSeconds)
	if err != nil {
		return domain.WorkEntry{}, err
	}
	entry, err := s.repo.CreateWorkEntry(ctx, domain.WorkEntry{
		ProjectID: projectID, WorkDate: date, Description: description, DurationSeconds: durationSeconds,
		HourlyRateKopecks: project.HourlyRateKopecks, AmountKopecks: amount,
	})
	if err == nil {
		entry.ProjectName = project.Name
	}
	return entry, err
}

func (s *Service) GetWorkEntry(ctx context.Context, id int64) (domain.WorkEntry, error) {
	return s.repo.GetWorkEntry(ctx, id)
}

func (s *Service) ListWorkEntries(ctx context.Context, month time.Time, projectID int64, limit, offset int) ([]domain.WorkEntry, int, error) {
	start, end := MonthBounds(month, s.location)
	entries, err := s.repo.ListWorkEntries(ctx, start, end, projectID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	count, err := s.repo.CountWorkEntries(ctx, start, end, projectID)
	return entries, count, err
}

func (s *Service) AllWorkEntries(ctx context.Context, month time.Time, projectID int64) ([]domain.WorkEntry, error) {
	start, end := MonthBounds(month, s.location)
	return s.repo.ListWorkEntries(ctx, start, end, projectID, 10000, 0)
}

func (s *Service) UpdateDescription(ctx context.Context, id int64, description string) (domain.WorkEntry, error) {
	description, err := validateText(description, MaxDescriptionLength, "описание")
	if err != nil {
		return domain.WorkEntry{}, err
	}
	return s.repo.UpdateWorkDescription(ctx, id, description)
}

func (s *Service) UpdateDate(ctx context.Context, id int64, date time.Time) (domain.WorkEntry, error) {
	date, err := s.ValidateWorkDate(date)
	if err != nil {
		return domain.WorkEntry{}, err
	}
	return s.repo.UpdateWorkDate(ctx, id, date)
}

func (s *Service) UpdateDuration(ctx context.Context, id, durationSeconds int64) (domain.WorkEntry, error) {
	entry, err := s.repo.GetWorkEntry(ctx, id)
	if err != nil {
		return domain.WorkEntry{}, err
	}
	amount, err := money.CalculateAmount(entry.HourlyRateKopecks, durationSeconds)
	if err != nil {
		return domain.WorkEntry{}, err
	}
	return s.repo.UpdateWorkDuration(ctx, id, durationSeconds, amount)
}

func (s *Service) DeleteWorkEntry(ctx context.Context, id int64) error {
	return s.repo.DeleteWorkEntry(ctx, id)
}

func (s *Service) MonthlyReport(ctx context.Context, month time.Time) (domain.MonthlyReport, error) {
	start, end := MonthBounds(month, s.location)
	projects, err := s.repo.MonthlyReport(ctx, start, end)
	if err != nil {
		return domain.MonthlyReport{}, err
	}
	report := domain.MonthlyReport{Month: start, Projects: projects}
	for _, project := range projects {
		report.EntryCount += project.EntryCount
		report.DurationSeconds += project.DurationSeconds
		report.AmountKopecks += project.AmountKopecks
	}
	return report, nil
}

func (s *Service) SetPaymentStatus(ctx context.Context, projectID int64, month time.Time, status string) error {
	if status != domain.PaymentPaid && status != domain.PaymentPending {
		return errors.New("неизвестный статус оплаты")
	}
	start, _ := MonthBounds(month, s.location)
	return s.repo.SetPaymentStatus(ctx, projectID, start, status)
}

func (s *Service) Today() time.Time {
	now := s.now().In(s.location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
}

func (s *Service) ValidateWorkDate(date time.Time) (time.Time, error) {
	local := date.In(s.location)
	date = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.location)
	if date.After(s.Today()) {
		return time.Time{}, errors.New("дата работы не может быть в будущем")
	}
	return date, nil
}

func (s *Service) ParseDate(value string) (time.Time, error) {
	date, err := time.ParseInLocation("02.01.2006", strings.TrimSpace(value), s.location)
	if err != nil {
		return time.Time{}, errors.New("неверная дата, используйте ДД.ММ.ГГГГ")
	}
	return s.ValidateWorkDate(date)
}

func ParseMonth(value string, location *time.Location) (time.Time, error) {
	month, err := time.ParseInLocation("2006-01", strings.TrimSpace(value), location)
	if err != nil {
		return time.Time{}, errors.New("неверный месяц, используйте ГГГГ-ММ")
	}
	return month, nil
}

func MonthBounds(month time.Time, location *time.Location) (time.Time, time.Time) {
	local := month.In(location)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	return start, start.AddDate(0, 1, 0)
}

func AggregateEntries(month time.Time, entries []domain.WorkEntry, location *time.Location) domain.MonthlyReport {
	start, _ := MonthBounds(month, location)
	report := domain.MonthlyReport{Month: start}
	for _, entry := range entries {
		report.EntryCount++
		report.DurationSeconds += entry.DurationSeconds
		report.AmountKopecks += entry.AmountKopecks
	}
	return report
}

func validateText(value string, maxLength int, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s не может быть пустым", field)
	}
	if utf8.RuneCountInString(value) > maxLength {
		return "", fmt.Errorf("%s слишком длинное (максимум %d символов)", field, maxLength)
	}
	return value, nil
}
