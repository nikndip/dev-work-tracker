package service

import (
	"context"
	"testing"
	"time"

	"dev-work-tracker/internal/domain"
)

type fakeRepo struct {
	project     domain.Project
	entries     map[int64]domain.WorkEntry
	nextID      int64
	reportStart time.Time
	reportEnd   time.Time
}

func (f *fakeRepo) ListProjects(context.Context, bool) ([]domain.Project, error) {
	return []domain.Project{f.project}, nil
}
func (f *fakeRepo) GetProject(context.Context, int64) (domain.Project, error) { return f.project, nil }
func (f *fakeRepo) CreateProject(_ context.Context, name string, rate int64) (domain.Project, error) {
	f.project = domain.Project{ID: 1, Name: name, HourlyRateKopecks: rate, IsActive: true}
	return f.project, nil
}
func (f *fakeRepo) UpdateProjectName(_ context.Context, _ int64, name string) (domain.Project, error) {
	f.project.Name = name
	return f.project, nil
}
func (f *fakeRepo) UpdateProjectRate(_ context.Context, _ int64, rate int64) (domain.Project, error) {
	f.project.HourlyRateKopecks = rate
	return f.project, nil
}
func (f *fakeRepo) DeactivateProject(context.Context, int64) (domain.Project, error) {
	f.project.IsActive = false
	return f.project, nil
}
func (f *fakeRepo) CreateWorkEntry(_ context.Context, e domain.WorkEntry) (domain.WorkEntry, error) {
	f.nextID++
	e.ID = f.nextID
	if f.entries == nil {
		f.entries = map[int64]domain.WorkEntry{}
	}
	f.entries[e.ID] = e
	return e, nil
}
func (f *fakeRepo) GetWorkEntry(_ context.Context, id int64) (domain.WorkEntry, error) {
	return f.entries[id], nil
}
func (f *fakeRepo) ListWorkEntries(context.Context, time.Time, time.Time, int64, int, int) ([]domain.WorkEntry, error) {
	return nil, nil
}
func (f *fakeRepo) CountWorkEntries(context.Context, time.Time, time.Time, int64) (int, error) {
	return 0, nil
}
func (f *fakeRepo) UpdateWorkDescription(_ context.Context, id int64, value string) (domain.WorkEntry, error) {
	e := f.entries[id]
	e.Description = value
	f.entries[id] = e
	return e, nil
}
func (f *fakeRepo) UpdateWorkDate(_ context.Context, id int64, value time.Time) (domain.WorkEntry, error) {
	e := f.entries[id]
	e.WorkDate = value
	f.entries[id] = e
	return e, nil
}
func (f *fakeRepo) UpdateWorkDuration(_ context.Context, id, seconds, amount int64) (domain.WorkEntry, error) {
	e := f.entries[id]
	e.DurationSeconds = seconds
	e.AmountKopecks = amount
	f.entries[id] = e
	return e, nil
}
func (f *fakeRepo) DeleteWorkEntry(_ context.Context, id int64) error {
	delete(f.entries, id)
	return nil
}
func (f *fakeRepo) MonthlyReport(_ context.Context, start, end time.Time) ([]domain.ProjectReport, error) {
	f.reportStart, f.reportEnd = start, end
	var result domain.ProjectReport
	result.ProjectID = 1
	result.ProjectName = "P"
	for _, e := range f.entries {
		if !e.WorkDate.Before(start) && e.WorkDate.Before(end) {
			result.EntryCount++
			result.DurationSeconds += e.DurationSeconds
			result.AmountKopecks += e.AmountKopecks
		}
	}
	if result.EntryCount == 0 {
		return nil, nil
	}
	return []domain.ProjectReport{result}, nil
}
func (f *fakeRepo) SetPaymentStatus(context.Context, int64, time.Time, string) error { return nil }

func newTestService(repo *fakeRepo) *Service {
	loc := time.FixedZone("Europe/Moscow", 3*60*60)
	s := New(repo, 200000, loc)
	s.now = func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, loc) }
	return s
}

func TestRateSnapshotAndDurationRecalculation(t *testing.T) {
	repo := &fakeRepo{project: domain.Project{ID: 1, Name: "Panorama", HourlyRateKopecks: 200000, IsActive: true}}
	s := newTestService(repo)
	workA, err := s.CreateWorkEntry(context.Background(), 1, s.Today(), "A", 3600)
	if err != nil {
		t.Fatal(err)
	}
	if workA.HourlyRateKopecks != 200000 || workA.AmountKopecks != 200000 {
		t.Fatalf("unexpected A: %+v", workA)
	}
	repo.project.HourlyRateKopecks = 250000
	workB, err := s.CreateWorkEntry(context.Background(), 1, s.Today(), "B", 3600)
	if err != nil {
		t.Fatal(err)
	}
	if repo.entries[workA.ID].AmountKopecks != 200000 || workB.AmountKopecks != 250000 {
		t.Fatal("existing entry changed or new rate was not snapshotted")
	}
	updated, err := s.UpdateDuration(context.Background(), workA.ID, 4800)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AmountKopecks != 266667 {
		t.Fatalf("got %d, want 266667", updated.AmountKopecks)
	}
}

func TestMonthlyReportBoundariesAndExactSum(t *testing.T) {
	loc := time.FixedZone("Europe/Moscow", 3*60*60)
	repo := &fakeRepo{entries: map[int64]domain.WorkEntry{
		1: {WorkDate: time.Date(2026, 8, 31, 0, 0, 0, 0, loc), DurationSeconds: 1, AmountKopecks: 999999},
		2: {WorkDate: time.Date(2026, 9, 1, 0, 0, 0, 0, loc), DurationSeconds: 1800, AmountKopecks: 100000},
		3: {WorkDate: time.Date(2026, 9, 15, 0, 0, 0, 0, loc), DurationSeconds: 3000, AmountKopecks: 166667},
		4: {WorkDate: time.Date(2026, 9, 30, 0, 0, 0, 0, loc), DurationSeconds: 5400, AmountKopecks: 300000},
		5: {WorkDate: time.Date(2026, 10, 1, 0, 0, 0, 0, loc), DurationSeconds: 1, AmountKopecks: 999999},
	}}
	s := newTestService(repo)
	report, err := s.MonthlyReport(context.Background(), time.Date(2026, 9, 10, 0, 0, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if report.EntryCount != 3 || report.AmountKopecks != 566667 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if repo.reportStart.Day() != 1 || repo.reportStart.Month() != time.September || repo.reportEnd.Month() != time.October || repo.reportEnd.Day() != 1 {
		t.Fatalf("unexpected bounds %v..%v", repo.reportStart, repo.reportEnd)
	}
}

func TestMonthlyReportSumsIndividuallyRoundedEntries(t *testing.T) {
	loc := time.FixedZone("Europe/Moscow", 3*60*60)
	minutes := []int64{65, 83, 57, 40, 51, 45, 35, 50, 60, 32, 30, 57, 90}
	amounts := []int64{216667, 276667, 190000, 133333, 170000, 150000, 116667, 166667, 200000, 106667, 100000, 190000, 300000}
	entries := make(map[int64]domain.WorkEntry, len(minutes))
	for i := range minutes {
		entries[int64(i+1)] = domain.WorkEntry{
			WorkDate: time.Date(2026, 9, i+1, 0, 0, 0, 0, loc),
			DurationSeconds: minutes[i] * 60,
			AmountKopecks: amounts[i],
		}
	}

	report, err := newTestService(&fakeRepo{entries: entries}).MonthlyReport(context.Background(), time.Date(2026, 9, 1, 0, 0, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if report.EntryCount != 13 || report.DurationSeconds != 41700 || report.AmountKopecks != 2316668 {
		t.Fatalf("unexpected report totals: %+v", report)
	}
	if report.AmountKopecks == (200000*report.DurationSeconds+1800)/3600 {
		t.Fatal("monthly amount must be the sum of individually rounded entries")
	}
}

func TestTodayAndYesterdayUseMoscowCalendarDate(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name          string
		now           time.Time
		wantToday     string
		wantYesterday string
	}{
		{"regular date after midnight", time.Date(2026, 9, 26, 0, 10, 0, 0, location), "26.09.2026", "25.09.2026"},
		{"month boundary", time.Date(2026, 10, 1, 0, 10, 0, 0, location), "01.10.2026", "30.09.2026"},
		{"year boundary", time.Date(2027, 1, 1, 0, 10, 0, 0, location), "01.01.2027", "31.12.2026"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := New(&fakeRepo{}, 200000, location)
			s.now = func() time.Time { return test.now }
			if got := s.Today().Format("02.01.2006"); got != test.wantToday {
				t.Fatalf("Today() = %s, want %s", got, test.wantToday)
			}
			if got := s.Yesterday().Format("02.01.2006"); got != test.wantYesterday {
				t.Fatalf("Yesterday() = %s, want %s", got, test.wantYesterday)
			}
		})
	}
}
