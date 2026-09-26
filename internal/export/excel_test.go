package export

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"dev-work-tracker/internal/domain"

	"github.com/xuri/excelize/v2"
)

func TestBuildExcelLightBusinessLayout(t *testing.T) {
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	longDescription := strings.Repeat("Длинное описание выполненной работы для проверки переноса текста. ", 8)
	result, err := BuildExcel(month, "", []domain.WorkEntry{
		{ID: 1, ProjectName: "Panorama CRM", WorkDate: month, Description: longDescription, DurationSeconds: 4800, HourlyRateKopecks: 200000, AmountKopecks: 266667},
		{ID: 2, ProjectName: "ProjectHub", WorkDate: month.AddDate(0, 0, 1), Description: "Вторая работа", DurationSeconds: 9000, HourlyRateKopecks: 200000, AmountKopecks: 500000},
	}, time.Date(2026, 9, 26, 20, 53, 0, 0, time.UTC))
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

	if got, _ := f.GetCellValue("Отчёт", "A4"); got != "Дата формирования" {
		t.Fatalf("metadata label is missing or truncated: %q", got)
	}
	if got, _ := f.GetCellValue("Отчёт", "C3"); got != "Сентябрь 2026" {
		t.Fatalf("unexpected period: %q", got)
	}
	if got, _ := f.GetCellValue("Отчёт", "D7"); got != longDescription {
		t.Fatal("long description was changed or lost")
	}

	header := cellStyle(t, f, "B6")
	assertFill(t, header, "4472C4")
	if header.Font == nil || !header.Font.Bold || !sameColor(header.Font.Color, "FFFFFF") {
		t.Fatalf("header must have bold white font: %+v", header.Font)
	}
	body := cellStyle(t, f, "D7")
	assertFill(t, body, "FFFFFF")
	if body.Font == nil || sameColor(body.Font.Color, "FFFFFF") || sameColor(body.Font.Color, "00000000") {
		t.Fatalf("body font must be explicitly dark: %+v", body.Font)
	}
	if body.Alignment == nil || !body.Alignment.WrapText {
		t.Fatal("description column must have WrapText")
	}
	total := cellStyle(t, f, "D10")
	assertFill(t, total, "DDEBF7")
	if total.Font == nil || sameColor(total.Font.Color, "FFFFFF") {
		t.Fatalf("totals must use dark text: %+v", total.Font)
	}

	if styleID, err := f.GetCellStyle("Отчёт", "H100"); err != nil || styleID != 0 {
		t.Fatalf("unused worksheet area must remain unstyled, style=%d err=%v", styleID, err)
	}
	for column, want := range map[string]float64{"A": 7, "B": 14, "C": 26, "D": 55, "E": 16, "F": 18, "G": 19} {
		got, err := f.GetColWidth("Отчёт", column)
		if err != nil || got != want {
			t.Errorf("column %s width = %v, %v; want %v", column, got, err, want)
		}
	}
	widthA, _ := f.GetColWidth("Отчёт", "A")
	widthB, _ := f.GetColWidth("Отчёт", "B")
	if widthA+widthB < 20 {
		t.Fatalf("metadata label area is too narrow: %.1f", widthA+widthB)
	}
	if height, _ := f.GetRowHeight("Отчёт", 7); height <= 21 {
		t.Fatalf("long description row was not expanded: %.1f", height)
	}

	if formula, _ := f.GetCellFormula("Отчёт", "F7"); formula != "=200000/100" {
		t.Fatalf("hourly rate changed: %q", formula)
	}
	if formula, _ := f.GetCellFormula("Отчёт", "G7"); formula != "=266667/100" {
		t.Fatalf("entry amount changed: %q", formula)
	}
	if formula, _ := f.GetCellFormula("Отчёт", "G12"); formula != "=766667/100" {
		t.Fatalf("monthly total changed: %q", formula)
	}
	money := cellStyle(t, f, "G7")
	if money.CustomNumFmt == nil || *money.CustomNumFmt != "#,##0.00" {
		t.Fatalf("unexpected money number format: %+v", money.CustomNumFmt)
	}

	panes, err := f.GetPanes("Отчёт")
	if err != nil || !panes.Freeze || panes.YSplit != 6 || panes.TopLeftCell != "A7" {
		t.Fatalf("unexpected freeze panes: %+v, %v", panes, err)
	}
}

func cellStyle(t *testing.T, f *excelize.File, cell string) *excelize.Style {
	t.Helper()
	id, err := f.GetCellStyle("Отчёт", cell)
	if err != nil {
		t.Fatal(err)
	}
	style, err := f.GetStyle(id)
	if err != nil {
		t.Fatal(err)
	}
	return style
}

func assertFill(t *testing.T, style *excelize.Style, want string) {
	t.Helper()
	if style.Fill.Type != "pattern" || style.Fill.Pattern != 1 || len(style.Fill.Color) == 0 || !sameColor(style.Fill.Color[0], want) {
		t.Fatalf("fill = %+v, want solid %s", style.Fill, want)
	}
}

func sameColor(got, want string) bool {
	got = strings.ToUpper(strings.TrimPrefix(got, "#"))
	want = strings.ToUpper(strings.TrimPrefix(want, "#"))
	return got == want || strings.HasSuffix(got, want)
}
