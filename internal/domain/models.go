package domain

import "time"

type Project struct {
	ID                int64
	Name              string
	HourlyRateKopecks int64
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type WorkEntry struct {
	ID                int64
	ProjectID         int64
	ProjectName       string
	WorkDate          time.Time
	Description       string
	DurationSeconds   int64
	HourlyRateKopecks int64
	AmountKopecks     int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ProjectReport struct {
	ProjectID       int64
	ProjectName     string
	EntryCount      int64
	DurationSeconds int64
	AmountKopecks   int64
	PaymentStatus   string
	PaidAt          *time.Time
}

type MonthlyReport struct {
	Month           time.Time
	Projects        []ProjectReport
	EntryCount      int64
	DurationSeconds int64
	AmountKopecks   int64
}

const (
	PaymentPending = "pending"
	PaymentPaid    = "paid"
)
