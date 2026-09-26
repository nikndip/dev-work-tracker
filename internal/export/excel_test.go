package export

import (
	"bytes"
	"testing"
	"time"

	"dev-work-tracker/internal/domain"

	"github.com/xuri/excelize/v2"
)

func TestBuildExcel(t *testing.T) {
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	result, err := BuildExcel(month, "", []domain.WorkEntry{{
		ID: 1, ProjectName: "Panorama CRM", WorkDate: month, Description: "Исправил расчёт", DurationSeconds: 4800,
		HourlyRateKopecks: 200000, AmountKopecks: 266667,
	}}, month)
	if err != nil {
		t.Fatal(err)
	}
	if result.Filename != "Dev_Work_Tracker_2026-09.xlsx" {
		t.Fatalf("unexpected filename: %s", result.Filename)
	}
	f, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got, _ := f.GetCellValue("Отчёт", "D7"); got != "Исправил расчёт" {
		t.Fatalf("unexpected description: %q", got)
	}
	if formula, _ := f.GetCellFormula("Отчёт", "G7"); formula != "=266667/100" {
		t.Fatalf("money must use exact kopecks formula, got %q", formula)
	}
}
