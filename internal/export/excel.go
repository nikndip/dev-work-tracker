package export

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"dev-work-tracker/internal/domain"
	"dev-work-tracker/internal/duration"

	"github.com/xuri/excelize/v2"
)

type Excel struct {
	Data     []byte
	Filename string
}

func BuildExcel(month time.Time, projectName string, entries []domain.WorkEntry, generatedAt time.Time) (Excel, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Отчёт"
	f.SetSheetName("Sheet1", sheet)

	lightBorder := []excelize.Border{
		{Type: "left", Color: "D9E2F3", Style: 1}, {Type: "right", Color: "D9E2F3", Style: 1},
		{Type: "top", Color: "D9E2F3", Style: 1}, {Type: "bottom", Color: "D9E2F3", Style: 1},
	}
	whiteFill := excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1}
	totalFill := excelize.Fill{Type: "pattern", Color: []string{"DDEBF7"}, Pattern: 1}
	darkFont := &excelize.Font{Family: "Arial", Size: 10, Color: "1F2937"}
	moneyFormat := "#,##0.00"
	integerFormat := "0"

	worksheetStyle, err := f.NewStyle(&excelize.Style{Font: darkFont, Fill: whiteFill})
	if err != nil {
		return Excel{}, fmt.Errorf("create worksheet style: %w", err)
	}
	// Set the light default before creating rows. Excelize then writes one
	// compact column-style range instead of materializing millions of cells.
	if err := f.SetColStyle(sheet, "A:XFD", worksheetStyle); err != nil {
		return Excel{}, fmt.Errorf("set worksheet style: %w", err)
	}

	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Bold: true, Size: 16, Color: "1F4E78"}, Fill: whiteFill, Alignment: &excelize.Alignment{Vertical: "center"}})
	metadataBackgroundStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: whiteFill, Alignment: &excelize.Alignment{Vertical: "center"}})
	labelStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Bold: true, Size: 10, Color: "1F4E78"}, Fill: whiteFill, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"}})
	metadataValueStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: whiteFill, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"}})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Arial", Bold: true, Size: 10, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    lightBorder,
	})
	bodyLeftStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: whiteFill, Border: lightBorder, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"}})
	bodyCenterStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: whiteFill, Border: lightBorder, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	integerStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: whiteFill, Border: lightBorder, CustomNumFmt: &integerFormat, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	descriptionStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: whiteFill, Border: lightBorder, Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "top", WrapText: true}})
	moneyStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: whiteFill, Border: lightBorder, CustomNumFmt: &moneyFormat, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"}})
	totalStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: totalFill, Border: lightBorder, Alignment: &excelize.Alignment{Vertical: "center"}})
	totalIntegerStyle, _ := f.NewStyle(&excelize.Style{Font: darkFont, Fill: totalFill, Border: lightBorder, CustomNumFmt: &integerFormat, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	totalMoneyStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Bold: true, Size: 10, Color: "1F2937"}, Fill: totalFill, Border: lightBorder, CustomNumFmt: &moneyFormat, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"}})

	f.MergeCell(sheet, "A1", "H1")
	f.SetCellValue(sheet, "A1", "Dev Work Tracker")
	f.SetCellStyle(sheet, "A1", "H1", titleStyle)
	f.SetRowHeight(sheet, 1, 26)
	if projectName == "" {
		projectName = "Все проекты"
	}
	meta := [][2]string{{"Проект", projectName}, {"Период", formatMonth(month)}, {"Дата формирования", generatedAt.Format("02.01.2006 15:04")}}
	for i, item := range meta {
		row := i + 2
		f.MergeCell(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row))
		f.MergeCell(sheet, fmt.Sprintf("C%d", row), fmt.Sprintf("H%d", row))
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), item[0])
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), item[1])
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("H%d", row), metadataBackgroundStyle)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), labelStyle)
		f.SetCellStyle(sheet, fmt.Sprintf("C%d", row), fmt.Sprintf("H%d", row), metadataValueStyle)
		f.SetRowHeight(sheet, row, 20)
	}

	const headerRow = 6
	headers := []string{"№", "Дата", "Проект", "Выполненная работа", "Время", "Время, мин", "Ставка, ₽/ч", "Стоимость, ₽"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		f.SetCellValue(sheet, cell, header)
	}
	f.SetCellStyle(sheet, "A6", "H6", headerStyle)
	f.SetRowHeight(sheet, headerRow, 28)

	var totalSeconds, totalKopecks int64
	for i, entry := range entries {
		row := headerRow + 1 + i
		values := []any{i + 1, entry.WorkDate.Format("02.01.2006"), entry.ProjectName, entry.Description, duration.Format(entry.DurationSeconds)}
		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			f.SetCellValue(sheet, cell, value)
		}
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), duration.TotalMinutes(entry.DurationSeconds))
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), rublesValue(entry.HourlyRateKopecks))
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), rublesValue(entry.AmountKopecks))
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), bodyCenterStyle)
		f.SetCellStyle(sheet, fmt.Sprintf("C%d", row), fmt.Sprintf("C%d", row), bodyLeftStyle)
		f.SetCellStyle(sheet, fmt.Sprintf("D%d", row), fmt.Sprintf("D%d", row), descriptionStyle)
		f.SetCellStyle(sheet, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), bodyCenterStyle)
		f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("F%d", row), integerStyle)
		f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("H%d", row), moneyStyle)
		f.SetRowHeight(sheet, row, descriptionRowHeight(entry.Description))
		totalSeconds += entry.DurationSeconds
		totalKopecks += entry.AmountKopecks
	}

	totalRow := headerRow + len(entries) + 2
	f.SetCellValue(sheet, fmt.Sprintf("D%d", totalRow), "Количество работ")
	f.SetCellValue(sheet, fmt.Sprintf("E%d", totalRow), len(entries))
	f.SetCellValue(sheet, fmt.Sprintf("D%d", totalRow+1), "Общее время")
	f.SetCellValue(sheet, fmt.Sprintf("E%d", totalRow+1), duration.Format(totalSeconds))
	f.SetCellValue(sheet, fmt.Sprintf("D%d", totalRow+2), "Всего минут")
	f.SetCellValue(sheet, fmt.Sprintf("E%d", totalRow+2), duration.TotalMinutes(totalSeconds))
	f.SetCellValue(sheet, fmt.Sprintf("D%d", totalRow+3), "Итого, ₽")
	f.SetCellValue(sheet, fmt.Sprintf("H%d", totalRow+3), rublesValue(totalKopecks))
	f.SetCellStyle(sheet, fmt.Sprintf("D%d", totalRow), fmt.Sprintf("H%d", totalRow+3), totalStyle)
	f.SetCellStyle(sheet, fmt.Sprintf("E%d", totalRow), fmt.Sprintf("E%d", totalRow), totalIntegerStyle)
	f.SetCellStyle(sheet, fmt.Sprintf("E%d", totalRow+2), fmt.Sprintf("E%d", totalRow+2), totalIntegerStyle)
	f.SetCellStyle(sheet, fmt.Sprintf("H%d", totalRow+3), fmt.Sprintf("H%d", totalRow+3), totalMoneyStyle)
	for row := totalRow; row <= totalRow+3; row++ {
		f.SetRowHeight(sheet, row, 21)
	}

	widths := map[string]float64{"A": 7, "B": 14, "C": 26, "D": 55, "E": 16, "F": 14, "G": 18, "H": 19}
	for col, width := range widths {
		f.SetColWidth(sheet, col, col, width)
	}
	f.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: headerRow, TopLeftCell: "A7", ActivePane: "bottomLeft"})
	f.AutoFilter(sheet, fmt.Sprintf("A%d:H%d", headerRow, headerRow+len(entries)), nil)
	view := "normal"
	showGridLines := true
	zoom := 100.0
	f.SetSheetView(sheet, 0, &excelize.ViewOptions{View: &view, ShowGridLines: &showGridLines, ZoomScale: &zoom})
	fitToPage := true
	f.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: &fitToPage})
	pageSize, fitWidth, fitHeight := 9, 1, 0
	orientation := "landscape"
	f.SetPageLayout(sheet, &excelize.PageLayoutOptions{Size: &pageSize, Orientation: &orientation, FitToWidth: &fitWidth, FitToHeight: &fitHeight})

	buffer, err := f.WriteToBuffer()
	if err != nil {
		return Excel{}, fmt.Errorf("write xlsx: %w", err)
	}
	return Excel{Data: bytes.Clone(buffer.Bytes()), Filename: filename(projectName, month)}, nil
}

func rublesValue(kopecks int64) float64 {
	return float64(kopecks) / 100
}

var russianMonths = [...]string{"", "Январь", "Февраль", "Март", "Апрель", "Май", "Июнь", "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"}

func formatMonth(month time.Time) string {
	return fmt.Sprintf("%s %d", russianMonths[month.Month()], month.Year())
}

func descriptionRowHeight(value string) float64 {
	const (
		charactersPerLine = 55
		baseHeight        = 21.0
		lineHeight        = 15.0
		maxHeight         = 300.0
	)
	lines := 0
	for _, part := range strings.Split(value, "\n") {
		partLines := (utf8.RuneCountInString(part) + charactersPerLine - 1) / charactersPerLine
		if partLines < 1 {
			partLines = 1
		}
		lines += partLines
	}
	height := baseHeight + float64(lines-1)*lineHeight
	if height > maxHeight {
		return maxHeight
	}
	return height
}

var unsafeFilename = regexp.MustCompile(`[^\pL\pN_-]+`)

func filename(projectName string, month time.Time) string {
	base := "Dev_Work_Tracker"
	if projectName != "" && projectName != "Все проекты" {
		base = strings.Trim(unsafeFilename.ReplaceAllString(projectName, "_"), "_")
		if base == "" {
			base = "Project"
		}
	}
	return fmt.Sprintf("%s_%s.xlsx", base, month.Format("2006-01"))
}
