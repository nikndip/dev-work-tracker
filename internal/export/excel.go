package export

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

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

	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 18, Color: "1F4E78"}})
	labelStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "1F4E78"}})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4472C4"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    []excelize.Border{{Type: "left", Color: "D9E2F3", Style: 1}, {Type: "right", Color: "D9E2F3", Style: 1}, {Type: "top", Color: "D9E2F3", Style: 1}, {Type: "bottom", Color: "D9E2F3", Style: 1}},
	})
	bodyStyle, _ := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	moneyStyle, _ := f.NewStyle(&excelize.Style{NumFmt: 4, Alignment: &excelize.Alignment{Vertical: "top"}})
	totalStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Color: []string{"D9EAF7"}, Pattern: 1}})
	totalMoneyStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Color: []string{"D9EAF7"}, Pattern: 1}, NumFmt: 4})

	f.MergeCell(sheet, "A1", "G1")
	f.SetCellValue(sheet, "A1", "Dev Work Tracker")
	f.SetCellStyle(sheet, "A1", "A1", titleStyle)
	if projectName == "" {
		projectName = "Все проекты"
	}
	meta := [][2]string{{"Проект", projectName}, {"Период", month.Format("2006-01")}, {"Дата формирования", generatedAt.Format("02.01.2006 15:04")}}
	for i, item := range meta {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), item[0])
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), item[1])
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), labelStyle)
	}

	const headerRow = 6
	headers := []string{"№", "Дата", "Проект", "Выполненная работа", "Время", "Ставка, ₽/ч", "Стоимость, ₽"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, headerRow)
		f.SetCellValue(sheet, cell, header)
	}
	f.SetCellStyle(sheet, "A6", "G6", headerStyle)
	f.SetRowHeight(sheet, headerRow, 28)

	var totalSeconds, totalKopecks int64
	for i, entry := range entries {
		row := headerRow + 1 + i
		values := []any{i + 1, entry.WorkDate.Format("02.01.2006"), entry.ProjectName, entry.Description, duration.Format(entry.DurationSeconds)}
		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			f.SetCellValue(sheet, cell, value)
		}
		f.SetCellFormula(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("=%d/100", entry.HourlyRateKopecks))
		f.SetCellFormula(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("=%d/100", entry.AmountKopecks))
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), bodyStyle)
		f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("G%d", row), moneyStyle)
		totalSeconds += entry.DurationSeconds
		totalKopecks += entry.AmountKopecks
	}

	totalRow := headerRow + len(entries) + 2
	f.SetCellValue(sheet, fmt.Sprintf("D%d", totalRow), "Количество работ")
	f.SetCellValue(sheet, fmt.Sprintf("E%d", totalRow), len(entries))
	f.SetCellValue(sheet, fmt.Sprintf("D%d", totalRow+1), "Общее время")
	f.SetCellValue(sheet, fmt.Sprintf("E%d", totalRow+1), duration.Format(totalSeconds))
	f.SetCellValue(sheet, fmt.Sprintf("D%d", totalRow+2), "Итого, ₽")
	f.SetCellFormula(sheet, fmt.Sprintf("G%d", totalRow+2), fmt.Sprintf("=%d/100", totalKopecks))
	f.SetCellStyle(sheet, fmt.Sprintf("D%d", totalRow), fmt.Sprintf("G%d", totalRow+2), totalStyle)
	f.SetCellStyle(sheet, fmt.Sprintf("G%d", totalRow+2), fmt.Sprintf("G%d", totalRow+2), totalMoneyStyle)

	widths := map[string]float64{"A": 7, "B": 14, "C": 24, "D": 55, "E": 16, "F": 17, "G": 17}
	for col, width := range widths {
		f.SetColWidth(sheet, col, col, width)
	}
	f.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: false, YSplit: headerRow, TopLeftCell: "A7", ActivePane: "bottomLeft"})
	f.AutoFilter(sheet, fmt.Sprintf("A%d:G%d", headerRow, headerRow+len(entries)), nil)

	buffer, err := f.WriteToBuffer()
	if err != nil {
		return Excel{}, fmt.Errorf("write xlsx: %w", err)
	}
	return Excel{Data: bytes.Clone(buffer.Bytes()), Filename: filename(projectName, month)}, nil
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
