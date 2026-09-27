package export

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strconv"
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

	assertLightColumnStyle(t, f, "A")
	assertLightColumnStyle(t, f, "H")
	assertLightColumnStyle(t, f, "XFD")
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

	assertNumericMoney(t, f, "F7", "2000")
	assertNumericMoney(t, f, "G7", "2666.67")
	assertNumericMoney(t, f, "G12", "7666.67")
	money := cellStyle(t, f, "G7")
	if money.CustomNumFmt == nil || *money.CustomNumFmt != "#,##0.00" {
		t.Fatalf("unexpected money number format: %+v", money.CustomNumFmt)
	}

	panes, err := f.GetPanes("Отчёт")
	if err != nil || !panes.Freeze || panes.YSplit != 6 || panes.TopLeftCell != "A7" {
		t.Fatalf("unexpected freeze panes: %+v, %v", panes, err)
	}
}

func TestBuildExcelThirteenEntryFinancialRegression(t *testing.T) {
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	minutes := []int64{65, 83, 57, 40, 51, 45, 35, 50, 60, 32, 30, 57, 90}
	amounts := []int64{216667, 276667, 190000, 133333, 170000, 150000, 116667, 166667, 200000, 106667, 100000, 190000, 300000}
	entries := make([]domain.WorkEntry, len(minutes))
	for i := range minutes {
		entries[i] = domain.WorkEntry{
			ID: int64(i + 1), ProjectName: "ProjectHub", WorkDate: month.AddDate(0, 0, i),
			Description: "Работа", DurationSeconds: minutes[i] * 60,
			HourlyRateKopecks: 200000, AmountKopecks: amounts[i],
		}
	}

	result, err := BuildExcel(month, "ProjectHub", entries, month)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) > 250_000 {
		t.Fatalf("XLSX unexpectedly large: %d bytes", len(result.Data))
	}
	f, err := excelize.OpenReader(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for i, amount := range amounts {
		row := 7 + i
		assertNumericMoney(t, f, cell("F", row), "2000")
		assertNumericMoney(t, f, cell("G", row), rawRubles(amount))
	}
	assertNumericMoney(t, f, "G23", "23166.68")
	if got, _ := f.GetCellValue("Отчёт", "E21"); got != "13" {
		t.Fatalf("entry count = %q, want 13", got)
	}
	if got, _ := f.GetCellValue("Отчёт", "E22"); got != "11 ч 35 мин" {
		t.Fatalf("total duration = %q, want 11 ч 35 мин", got)
	}

	xml := worksheetXML(t, result.Data)
	if !strings.Contains(xml, `max="16384"`) {
		t.Fatal("worksheet is missing compact full-width column style")
	}
	if cells := strings.Count(xml, "<c "); cells > 500 {
		t.Fatalf("worksheet materialized too many cells: %d", cells)
	}
}

func assertNumericMoney(t *testing.T, f *excelize.File, address, want string) {
	t.Helper()
	if cellType, err := f.GetCellType("Отчёт", address); err != nil || (cellType != excelize.CellTypeNumber && cellType != excelize.CellTypeUnset) {
		t.Fatalf("%s type = %v, %v; want numeric", address, cellType, err)
	}
	if formula, _ := f.GetCellFormula("Отчёт", address); formula != "" {
		t.Fatalf("%s must contain a backend value, got formula %q", address, formula)
	}
	got, err := f.GetCellValue("Отчёт", address, excelize.Options{RawCellValue: true})
	if err != nil || got != want {
		t.Fatalf("%s raw value = %q, %v; want %q", address, got, err, want)
	}
}

func assertLightColumnStyle(t *testing.T, f *excelize.File, column string) {
	t.Helper()
	styleID, err := f.GetColStyle("Отчёт", column)
	if err != nil || styleID == 0 {
		t.Fatalf("column %s has no light default style: %d, %v", column, styleID, err)
	}
	style, err := f.GetStyle(styleID)
	if err != nil {
		t.Fatal(err)
	}
	assertFill(t, style, "FFFFFF")
	if style.Font == nil || sameColor(style.Font.Color, "FFFFFF") {
		t.Fatalf("column %s default font is not dark: %+v", column, style.Font)
	}
}

func worksheetXML(t *testing.T, data []byte) string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range archive.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	t.Fatal("worksheet XML not found")
	return ""
}

func cell(column string, row int) string {
	return column + fmt.Sprint(row)
}

func rawRubles(kopecks int64) string {
	return strconv.FormatFloat(float64(kopecks)/100, 'f', -1, 64)
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
