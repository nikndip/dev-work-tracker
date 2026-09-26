package telegram

import (
	"sync"
	"time"
)

type step string

const (
	stepIdle            step = ""
	stepProjectName     step = "project_name"
	stepProjectRate     step = "project_rate"
	stepAddDescription  step = "add_description"
	stepAddDate         step = "add_date"
	stepAddDuration     step = "add_duration"
	stepEditDescription step = "edit_description"
	stepEditDate        step = "edit_date"
	stepEditDuration    step = "edit_duration"
	stepProjectEditName step = "project_edit_name"
	stepProjectEditRate step = "project_edit_rate"
	stepReportMonth     step = "report_month"
	stepExportMonth     step = "export_month"
	stepPaymentMonth    step = "payment_month"
)

type session struct {
	Step          step
	ProjectID     int64
	ProjectName   string
	ProjectRate   int64
	Description   string
	WorkDate      time.Time
	Duration      int64
	EntryID       int64
	Month         time.Time
	ReturnToAdd   bool
	ReturnPreview bool
}

type sessionStore struct {
	mu   sync.Mutex
	data map[int64]session
}

func newSessionStore() *sessionStore            { return &sessionStore{data: make(map[int64]session)} }
func (s *sessionStore) get(id int64) session    { s.mu.Lock(); defer s.mu.Unlock(); return s.data[id] }
func (s *sessionStore) set(id int64, v session) { s.mu.Lock(); defer s.mu.Unlock(); s.data[id] = v }
func (s *sessionStore) clear(id int64)          { s.mu.Lock(); defer s.mu.Unlock(); delete(s.data, id) }
