package telegram

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"dev-work-tracker/internal/domain"
	"dev-work-tracker/internal/duration"
	"dev-work-tracker/internal/money"
	"github.com/go-telegram/bot/models"
)

func keyboard(rows ...[]models.InlineKeyboardButton) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}
func button(text, data string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: text, CallbackData: data}
}
func mainMenu() *models.InlineKeyboardMarkup {
	return keyboard(
		[]models.InlineKeyboardButton{button("➕ Добавить работу", "menu:new")},
		[]models.InlineKeyboardButton{button("📋 Мои работы", "menu:list"), button("📊 Отчёт", "menu:report")},
		[]models.InlineKeyboardButton{button("📤 Excel", "menu:export"), button("📁 Проекты", "menu:projects")},
		[]models.InlineKeyboardButton{button("💳 Оплаты", "menu:payments")},
	)
}
func menuText() string {
	return "Dev Work Tracker\n\nУчёт выполненных работ и рабочего времени."
}

var monthNames = [...]string{"", "Январь", "Февраль", "Март", "Апрель", "Май", "Июнь", "Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь"}

func monthTitle(month time.Time) string {
	return fmt.Sprintf("%s %d", monthNames[month.Month()], month.Year())
}
func previewText(s session) string {
	amount, _ := money.CalculateAmount(s.ProjectRate, s.Duration)
	return fmt.Sprintf("Проверьте запись:\n\nПроект: %s\nДата: %s\n\nРабота:\n%s\n\nВремя: %s\nСтавка: %s\nСтоимость: %s", s.ProjectName, s.WorkDate.Format("02.01.2006"), s.Description, duration.Format(s.Duration), money.FormatRate(s.ProjectRate), money.FormatKopecks(amount))
}
func entryText(e domain.WorkEntry, loc *time.Location) string {
	return fmt.Sprintf("Работа #%d\n\nПроект: %s\nДата: %s\n\nОписание:\n%s\n\nВремя: %s\nСтавка: %s\nСтоимость: %s\n\nСоздано: %s", e.ID, e.ProjectName, e.WorkDate.Format("02.01.2006"), e.Description, duration.Format(e.DurationSeconds), money.FormatRate(e.HourlyRateKopecks), money.FormatKopecks(e.AmountKopecks), e.CreatedAt.In(loc).Format("02.01.2006 15:04"))
}
func shortDescription(v string) string {
	v = strings.Join(strings.Fields(v), " ")
	if utf8.RuneCountInString(v) <= 45 {
		return v
	}
	return string([]rune(v)[:45]) + "…"
}
func reportText(r domain.MonthlyReport) string {
	var out strings.Builder
	fmt.Fprintf(&out, "📊 %s\n", monthTitle(r.Month))
	if len(r.Projects) == 0 {
		out.WriteString("\nЗа этот месяц работ пока нет.\n\n────────────────\nИтого:\nРабот: 0\nФактическое время: 0 мин\nСумма: 0 ₽")
		return out.String()
	}
	for _, p := range r.Projects {
		fmt.Fprintf(&out, "\n%s\nРабот: %d\nФактическое время: %s\nСумма: %s\n", p.ProjectName, p.EntryCount, duration.FormatReport(p.DurationSeconds), money.FormatKopecks(p.AmountKopecks))
	}
	out.WriteString("\n────────────────\nИтого:\n")
	fmt.Fprintf(&out, "%s\n%s", duration.FormatReport(r.DurationSeconds), money.FormatKopecks(r.AmountKopecks))
	return out.String()
}

func reportKeyboard(month, currentMonth time.Time) *models.InlineKeyboardMarkup {
	key := month.Format("2006-01")
	navigation := []models.InlineKeyboardButton{button("⬅️ Предыдущий месяц", "report:prev:"+key)}
	if month.Before(currentMonth) {
		navigation = append(navigation, button("Следующий месяц ➡️", "report:next:"+key))
	}
	return keyboard(
		navigation,
		[]models.InlineKeyboardButton{button("📅 Выбрать месяц", "report:choose")},
		[]models.InlineKeyboardButton{button("📋 Показать работы", "report:entries:"+key)},
		[]models.InlineKeyboardButton{button("🏠 Меню", "menu")},
	)
}

func nextReportMonth(month, currentMonth time.Time) (time.Time, bool) {
	next := month.AddDate(0, 1, 0)
	return next, !next.After(currentMonth)
}
