package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dev-work-tracker/internal/domain"
	"dev-work-tracker/internal/duration"
	workexport "dev-work-tracker/internal/export"
	"dev-work-tracker/internal/money"
	"dev-work-tracker/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"
)

const pageSize = 6

const (
	dateActionToday     = "today"
	dateActionYesterday = "yesterday"
)

func (b *Bot) handleUpdate(ctx context.Context, client *bot.Bot, update *models.Update) {
	userID, _ := updateUserID(update)
	if update.CallbackQuery != nil {
		_, _ = client.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID})
		chatID, ok := callbackChatID(update.CallbackQuery)
		if ok {
			b.handleCallback(ctx, client, userID, chatID, update.CallbackQuery.Data)
		}
		return
	}
	if update.Message == nil || update.Message.Text == "" {
		return
	}
	b.handleMessage(ctx, client, userID, update.Message.Chat.ID, update.Message.Text)
}

func callbackChatID(q *models.CallbackQuery) (int64, bool) {
	if q.Message.Message != nil {
		return q.Message.Message.Chat.ID, true
	}
	if q.Message.InaccessibleMessage != nil {
		return q.Message.InaccessibleMessage.Chat.ID, true
	}
	return 0, false
}

func (b *Bot) handleMessage(ctx context.Context, client *bot.Bot, userID, chatID int64, text string) {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "/") {
		command := strings.ToLower(strings.Fields(trimmed)[0])
		if at := strings.IndexByte(command, '@'); at >= 0 {
			command = command[:at]
		}
		switch command {
		case "/start":
			b.sessions.clear(userID)
			b.send(ctx, client, chatID, menuText(), mainMenu())
			return
		case "/cancel":
			b.sessions.clear(userID)
			b.send(ctx, client, chatID, "Текущий сценарий отменён.", mainMenu())
			return
		case "/new":
			b.startNew(ctx, client, userID, chatID)
			return
		case "/report":
			b.showReport(ctx, client, userID, chatID, b.service.Today())
			return
		case "/export":
			b.showExportOptions(ctx, client, userID, chatID, b.service.Today())
			return
		case "/projects":
			b.showProjects(ctx, client, userID, chatID)
			return
		case "/payments":
			b.showPayments(ctx, client, userID, chatID, b.service.Today())
			return
		}
	}
	s := b.sessions.get(userID)
	switch s.Step {
	case stepProjectName:
		s.ProjectName = trimmed
		s.Step = stepProjectRate
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, "Ставка проекта в рублях за час?\nНапример: 2000 или 2000,50", keyboard([]models.InlineKeyboardButton{button(money.FormatRate(b.service.DefaultRate()), "projectrate:default")}, []models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
	case stepProjectRate:
		b.finishProjectCreate(ctx, client, userID, chatID, s, trimmed)
	case stepAddDescription:
		if len([]rune(trimmed)) > service.MaxDescriptionLength || trimmed == "" {
			b.sendError(ctx, client, chatID, errors.New("описание должно содержать от 1 до 4000 символов"))
			return
		}
		s.Description = trimmed
		b.sessions.set(userID, s)
		b.askDate(ctx, client, chatID)
	case stepAddDate:
		date, err := b.service.ParseDate(trimmed)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		s = selectDraftDate(s, date)
		b.sessions.set(userID, s)
		b.askDuration(ctx, client, chatID)
	case stepAddDuration:
		seconds, err := duration.Parse(trimmed)
		if err != nil {
			b.sendDurationError(ctx, client, chatID)
			return
		}
		s.Duration = seconds
		s.Step = stepIdle
		b.sessions.set(userID, s)
		b.showPreview(ctx, client, chatID, s)
	case stepEditDescription:
		if s.ReturnPreview {
			s.Description = trimmed
			s.Step = stepIdle
			b.sessions.set(userID, s)
			b.showPreview(ctx, client, chatID, s)
			return
		}
		entry, err := b.service.UpdateDescription(ctx, s.EntryID, trimmed)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.sessions.clear(userID)
		b.showEntry(ctx, client, chatID, entry)
	case stepEditDate:
		date, err := b.service.ParseDate(trimmed)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		if s.ReturnPreview {
			s.WorkDate = date
			s.Step = stepIdle
			b.sessions.set(userID, s)
			b.showPreview(ctx, client, chatID, s)
			return
		}
		entry, err := b.service.UpdateDate(ctx, s.EntryID, date)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.sessions.clear(userID)
		b.showEntry(ctx, client, chatID, entry)
	case stepEditDuration:
		seconds, err := duration.Parse(trimmed)
		if err != nil {
			b.sendDurationError(ctx, client, chatID)
			return
		}
		if s.ReturnPreview {
			s.Duration = seconds
			s.Step = stepIdle
			b.sessions.set(userID, s)
			b.showPreview(ctx, client, chatID, s)
			return
		}
		entry, err := b.service.UpdateDuration(ctx, s.EntryID, seconds)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.sessions.clear(userID)
		b.showEntry(ctx, client, chatID, entry)
	case stepProjectEditName:
		project, err := b.service.UpdateProjectName(ctx, s.ProjectID, trimmed)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.sessions.clear(userID)
		b.showProject(ctx, client, chatID, project)
	case stepProjectEditRate:
		rate, err := money.ParseRubles(trimmed)
		if err != nil {
			b.sendError(ctx, client, chatID, errors.New("введите положительную ставку в рублях, например 2000 или 2000,50"))
			return
		}
		project, err := b.service.UpdateProjectRate(ctx, s.ProjectID, rate)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.sessions.clear(userID)
		b.showProject(ctx, client, chatID, project)
	case stepReportMonth:
		month, err := service.ParseMonth(trimmed, b.service.Location())
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.showReport(ctx, client, userID, chatID, month)
	case stepExportMonth:
		month, err := service.ParseMonth(trimmed, b.service.Location())
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.showExportOptions(ctx, client, userID, chatID, month)
	case stepPaymentMonth:
		month, err := service.ParseMonth(trimmed, b.service.Location())
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.showPayments(ctx, client, userID, chatID, month)
	default:
		b.send(ctx, client, chatID, "Выберите действие в меню.", mainMenu())
	}
}

func (b *Bot) handleCallback(ctx context.Context, client *bot.Bot, userID, chatID int64, data string) {
	if data == "cancel" || data == "menu" {
		b.sessions.clear(userID)
		b.send(ctx, client, chatID, menuText(), mainMenu())
		return
	}
	parts := strings.Split(data, ":")
	switch parts[0] {
	case "menu":
		if len(parts) < 2 {
			return
		}
		b.sessions.clear(userID)
		switch parts[1] {
		case "new":
			b.startNew(ctx, client, userID, chatID)
		case "list":
			b.showEntries(ctx, client, chatID, b.service.Today(), 0)
		case "report":
			b.showReport(ctx, client, userID, chatID, b.service.Today())
		case "export":
			b.showExportOptions(ctx, client, userID, chatID, b.service.Today())
		case "projects":
			b.showProjects(ctx, client, userID, chatID)
		case "payments":
			b.showPayments(ctx, client, userID, chatID, b.service.Today())
		}
	case "newproject":
		b.sessions.set(userID, session{Step: stepProjectName, ReturnToAdd: len(parts) > 1 && parts[1] == "add"})
		b.send(ctx, client, chatID, "Название проекта?", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
	case "projectrate":
		s := b.sessions.get(userID)
		if data == "projectrate:default" {
			defaultRate := b.service.DefaultRate()
			b.finishProjectCreate(ctx, client, userID, chatID, s, fmt.Sprintf("%d.%02d", defaultRate/100, defaultRate%100))
		}
	case "new":
		b.handleNewCallback(ctx, client, userID, chatID, parts)
	case "draft":
		b.handleDraftCallback(ctx, client, userID, chatID, parts)
	case "entries":
		if len(parts) == 3 {
			month, err := service.ParseMonth(parts[1], b.service.Location())
			page, err2 := strconv.Atoi(parts[2])
			if err == nil && err2 == nil {
				b.showEntries(ctx, client, chatID, month, page)
			}
		}
	case "entry":
		b.handleEntryCallback(ctx, client, userID, chatID, parts)
	case "report":
		b.handleReportCallback(ctx, client, userID, chatID, parts)
	case "export":
		b.handleExportCallback(ctx, client, userID, chatID, parts)
	case "projects":
		b.showProjects(ctx, client, userID, chatID)
	case "project":
		b.handleProjectCallback(ctx, client, userID, chatID, parts)
	case "payments":
		b.handlePaymentCallback(ctx, client, userID, chatID, parts)
	}
}

func (b *Bot) startNew(ctx context.Context, client *bot.Bot, userID, chatID int64) {
	projects, err := b.service.ListProjects(ctx, true)
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	if len(projects) == 0 {
		b.send(ctx, client, chatID, "У вас пока нет проектов.", keyboard([]models.InlineKeyboardButton{button("➕ Создать первый проект", "newproject:add")}, []models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
		return
	}
	rows := make([][]models.InlineKeyboardButton, 0, len(projects)+2)
	for _, p := range projects {
		rows = append(rows, []models.InlineKeyboardButton{button(p.Name, fmt.Sprintf("new:project:%d", p.ID))})
	}
	rows = append(rows, []models.InlineKeyboardButton{button("➕ Новый проект", "newproject:add")}, []models.InlineKeyboardButton{button("❌ Отмена", "cancel")})
	b.send(ctx, client, chatID, "Выберите проект:", keyboard(rows...))
}

func (b *Bot) finishProjectCreate(ctx context.Context, client *bot.Bot, userID, chatID int64, s session, value string) {
	rate, err := money.ParseRubles(value)
	if err != nil {
		b.sendError(ctx, client, chatID, errors.New("введите положительную ставку в рублях, например 2000 или 2000,50"))
		return
	}
	project, err := b.service.CreateProject(ctx, s.ProjectName, rate)
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	if s.ReturnToAdd {
		s = session{Step: stepAddDescription, ProjectID: project.ID, ProjectName: project.Name, ProjectRate: project.HourlyRateKopecks}
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, "Что было сделано?", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
		return
	}
	b.sessions.clear(userID)
	b.showProject(ctx, client, chatID, project)
}

func (b *Bot) handleNewCallback(ctx context.Context, client *bot.Bot, userID, chatID int64, parts []string) {
	if len(parts) >= 3 && parts[1] == "project" {
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return
		}
		p, err := b.service.GetProject(ctx, id)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.sessions.set(userID, session{Step: stepAddDescription, ProjectID: p.ID, ProjectName: p.Name, ProjectRate: p.HourlyRateKopecks})
		b.send(ctx, client, chatID, "Что было сделано?", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
		return
	}
	if len(parts) == 2 && parts[1] == "date" {
		s := b.sessions.get(userID)
		s.Step = stepAddDate
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, "Введите дату в формате ДД.ММ.ГГГГ:", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
		return
	}
	if len(parts) == 3 && parts[1] == "datevalue" {
		s := b.sessions.get(userID)
		selectedDate, err := b.dateForAction(parts[2])
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		s = selectDraftDate(s, selectedDate)
		b.sessions.set(userID, s)
		b.askDuration(ctx, client, chatID)
		return
	}
	if len(parts) == 2 && parts[1] == "duration" {
		s := b.sessions.get(userID)
		s.Step = stepAddDuration
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, durationHelp(), keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
		return
	}
	if len(parts) == 3 && parts[1] == "durationvalue" {
		seconds, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return
		}
		s := b.sessions.get(userID)
		s.Duration = seconds
		s.Step = stepIdle
		b.sessions.set(userID, s)
		b.showPreview(ctx, client, chatID, s)
	}
}

func (b *Bot) askDate(ctx context.Context, client *bot.Bot, chatID int64) {
	b.send(ctx, client, chatID, "Когда была выполнена работа?", dateKeyboard())
}

func dateKeyboard() *models.InlineKeyboardMarkup {
	return keyboard(
		[]models.InlineKeyboardButton{button("Сегодня", "new:datevalue:"+dateActionToday), button("Вчера", "new:datevalue:"+dateActionYesterday)},
		[]models.InlineKeyboardButton{button("📅 Ввести дату", "new:date")}, []models.InlineKeyboardButton{button("❌ Отмена", "cancel")})
}

func (b *Bot) dateForAction(action string) (time.Time, error) {
	switch action {
	case dateActionToday:
		return b.service.Today(), nil
	case dateActionYesterday:
		return b.service.Yesterday(), nil
	default:
		return time.Time{}, errors.New("неизвестный вариант даты")
	}
}

func selectDraftDate(s session, selectedDate time.Time) session {
	s.WorkDate = selectedDate
	s.Step = stepAddDuration
	return s
}
func (b *Bot) askDuration(ctx context.Context, client *bot.Bot, chatID int64) {
	b.send(ctx, client, chatID, "Сколько времени заняла работа?", keyboard(
		[]models.InlineKeyboardButton{button("30 мин", "new:durationvalue:1800"), button("1 час", "new:durationvalue:3600")},
		[]models.InlineKeyboardButton{button("1 ч 30 мин", "new:durationvalue:5400"), button("2 часа", "new:durationvalue:7200")},
		[]models.InlineKeyboardButton{button("✏️ Ввести вручную", "new:duration")}, []models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
}
func durationHelp() string {
	return "Введите длительность.\n\nПримеры:\n• 90 мин\n• 1 час 30 мин\n• 1:30\n• 1,5 часа"
}
func (b *Bot) sendDurationError(ctx context.Context, client *bot.Bot, chatID int64) {
	b.send(ctx, client, chatID, "Не удалось распознать время.\n\nПримеры:\n• 90 мин\n• 1 час 30 мин\n• 1:30\n• 1,5 часа", nil)
}
func (b *Bot) showPreview(ctx context.Context, client *bot.Bot, chatID int64, s session) {
	b.send(ctx, client, chatID, previewText(s), keyboard([]models.InlineKeyboardButton{button("✅ Сохранить", "draft:save"), button("✏️ Изменить", "draft:edit")}, []models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
}

func (b *Bot) handleDraftCallback(ctx context.Context, client *bot.Bot, userID, chatID int64, parts []string) {
	if len(parts) < 2 {
		return
	}
	s := b.sessions.get(userID)
	switch parts[1] {
	case "save":
		if s.ProjectID == 0 || s.Description == "" || s.Duration == 0 || s.WorkDate.IsZero() {
			b.send(ctx, client, chatID, "Черновик уже сохранён или отменён.", mainMenu())
			return
		}
		b.sessions.clear(userID)
		entry, err := b.service.CreateWorkEntry(ctx, s.ProjectID, s.WorkDate, s.Description, s.Duration)
		if err != nil {
			b.sessions.set(userID, s)
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.send(ctx, client, chatID, fmt.Sprintf("✅ Работа сохранена\n\nПроект: %s\nДата: %s\nВремя: %s\nСтоимость: %s", entry.ProjectName, entry.WorkDate.Format("02.01.2006"), duration.Format(entry.DurationSeconds), money.FormatKopecks(entry.AmountKopecks)), mainMenu())
	case "edit":
		b.send(ctx, client, chatID, "Что изменить?", keyboard([]models.InlineKeyboardButton{button("✏️ Описание", "draft:description"), button("📅 Дата", "draft:date")}, []models.InlineKeyboardButton{button("⏱ Время", "draft:duration"), button("⬅️ Назад", "draft:back")}))
	case "description":
		s.Step = stepEditDescription
		s.ReturnPreview = true
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, "Введите новое описание:", nil)
	case "date":
		s.Step = stepEditDate
		s.ReturnPreview = true
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, "Введите новую дату ДД.ММ.ГГГГ:", nil)
	case "duration":
		s.Step = stepEditDuration
		s.ReturnPreview = true
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, durationHelp(), nil)
	case "back":
		b.showPreview(ctx, client, chatID, s)
	}
}

func (b *Bot) showEntries(ctx context.Context, client *bot.Bot, chatID int64, month time.Time, page int) {
	if page < 0 {
		page = 0
	}
	entries, count, err := b.service.ListWorkEntries(ctx, month, 0, pageSize, page*pageSize)
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	var out strings.Builder
	fmt.Fprintf(&out, "📋 %s\n", monthTitle(month))
	if len(entries) == 0 {
		out.WriteString("\nРабот пока нет.")
	}
	rows := make([][]models.InlineKeyboardButton, 0, len(entries)+2)
	for _, e := range entries {
		fmt.Fprintf(&out, "\n%s\n#%d %s\n%s · %s\n", e.WorkDate.Format("02.01"), e.ID, shortDescription(e.Description), duration.Format(e.DurationSeconds), money.FormatKopecks(e.AmountKopecks))
		rows = append(rows, []models.InlineKeyboardButton{button(fmt.Sprintf("#%d · %s", e.ID, shortDescription(e.Description)), fmt.Sprintf("entry:view:%d", e.ID))})
	}
	var nav []models.InlineKeyboardButton
	monthKey := month.Format("2006-01")
	if page > 0 {
		nav = append(nav, button("⬅️", fmt.Sprintf("entries:%s:%d", monthKey, page-1)))
	}
	if (page+1)*pageSize < count {
		nav = append(nav, button("➡️", fmt.Sprintf("entries:%s:%d", monthKey, page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []models.InlineKeyboardButton{button("🏠 Меню", "menu")})
	b.send(ctx, client, chatID, out.String(), keyboard(rows...))
}

func (b *Bot) showEntry(ctx context.Context, client *bot.Bot, chatID int64, entry domain.WorkEntry) {
	b.send(ctx, client, chatID, entryText(entry, b.service.Location()), keyboard(
		[]models.InlineKeyboardButton{button("✏️ Описание", fmt.Sprintf("entry:description:%d", entry.ID)), button("📅 Дата", fmt.Sprintf("entry:date:%d", entry.ID))},
		[]models.InlineKeyboardButton{button("⏱ Время", fmt.Sprintf("entry:duration:%d", entry.ID)), button("🗑 Удалить", fmt.Sprintf("entry:delete:%d", entry.ID))},
		[]models.InlineKeyboardButton{button("⬅️ Назад", "menu:list")}))
}

func (b *Bot) handleEntryCallback(ctx context.Context, client *bot.Bot, userID, chatID int64, parts []string) {
	if len(parts) < 3 {
		return
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return
	}
	switch parts[1] {
	case "view":
		entry, err := b.service.GetWorkEntry(ctx, id)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.showEntry(ctx, client, chatID, entry)
	case "description":
		b.sessions.set(userID, session{Step: stepEditDescription, EntryID: id})
		b.send(ctx, client, chatID, "Введите новое описание:", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
	case "date":
		b.sessions.set(userID, session{Step: stepEditDate, EntryID: id})
		b.send(ctx, client, chatID, "Введите новую дату ДД.ММ.ГГГГ:", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
	case "duration":
		b.sessions.set(userID, session{Step: stepEditDuration, EntryID: id})
		b.send(ctx, client, chatID, durationHelp(), keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
	case "delete":
		b.send(ctx, client, chatID, "Удалить эту запись?", keyboard([]models.InlineKeyboardButton{button("Да, удалить", fmt.Sprintf("entry:deleteconfirm:%d", id)), button("Отмена", fmt.Sprintf("entry:view:%d", id))}))
	case "deleteconfirm":
		err := b.service.DeleteWorkEntry(ctx, id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.sessions.clear(userID)
		b.send(ctx, client, chatID, "Работа удалена.", mainMenu())
	}
}

func (b *Bot) showReport(ctx context.Context, client *bot.Bot, userID, chatID int64, month time.Time) {
	report, err := b.service.MonthlyReport(ctx, month)
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	b.sessions.set(userID, session{Month: report.Month})
	currentMonth, _ := service.MonthBounds(b.service.Today(), b.service.Location())
	b.send(ctx, client, chatID, reportText(report), reportKeyboard(report.Month, currentMonth))
}

func (b *Bot) handleReportCallback(ctx context.Context, client *bot.Bot, userID, chatID int64, parts []string) {
	if len(parts) < 2 {
		return
	}
	switch parts[1] {
	case "choose":
		s := b.sessions.get(userID)
		s.Step = stepReportMonth
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, "Введите месяц в формате ГГГГ-ММ:", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
	case "prev":
		if len(parts) == 3 {
			month, err := service.ParseMonth(parts[2], b.service.Location())
			if err == nil {
				b.showReport(ctx, client, userID, chatID, month.AddDate(0, -1, 0))
			}
		}
	case "next":
		if len(parts) == 3 {
			month, err := service.ParseMonth(parts[2], b.service.Location())
			if err != nil {
				return
			}
			currentMonth, _ := service.MonthBounds(b.service.Today(), b.service.Location())
			if next, ok := nextReportMonth(month, currentMonth); ok {
				b.showReport(ctx, client, userID, chatID, next)
			}
		}
	case "entries":
		if len(parts) == 3 {
			month, err := service.ParseMonth(parts[2], b.service.Location())
			if err == nil {
				b.showEntries(ctx, client, chatID, month, 0)
			}
		}
	}
}

func (b *Bot) showExportOptions(ctx context.Context, client *bot.Bot, userID, chatID int64, month time.Time) {
	projects, err := b.service.ListProjects(ctx, false)
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	start, _ := service.MonthBounds(month, b.service.Location())
	b.sessions.set(userID, session{Month: start})
	rows := [][]models.InlineKeyboardButton{{button("Все проекты", fmt.Sprintf("export:run:%s:0", start.Format("2006-01")))}}
	for _, p := range projects {
		rows = append(rows, []models.InlineKeyboardButton{button(p.Name, fmt.Sprintf("export:run:%s:%d", start.Format("2006-01"), p.ID))})
	}
	rows = append(rows, []models.InlineKeyboardButton{button("📅 Выбрать месяц", "export:choose")}, []models.InlineKeyboardButton{button("🏠 Меню", "menu")})
	b.send(ctx, client, chatID, "📤 Excel\n\nПериод: "+monthTitle(start)+"\nВыберите проект:", keyboard(rows...))
}

func (b *Bot) handleExportCallback(ctx context.Context, client *bot.Bot, userID, chatID int64, parts []string) {
	if len(parts) < 2 {
		return
	}
	if parts[1] == "choose" {
		s := b.sessions.get(userID)
		s.Step = stepExportMonth
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, "Введите месяц в формате ГГГГ-ММ:", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
		return
	}
	if parts[1] != "run" || len(parts) != 4 {
		return
	}
	month, err := service.ParseMonth(parts[2], b.service.Location())
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	projectID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return
	}
	projectName := ""
	if projectID > 0 {
		project, err := b.service.GetProject(ctx, projectID)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		projectName = project.Name
	}
	entries, err := b.service.AllWorkEntries(ctx, month, projectID)
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	if len(entries) == 0 {
		b.send(ctx, client, chatID, "За выбранный месяц работ нет — файл не создан.", mainMenu())
		return
	}
	// Repository returns DESC for the interactive list; Excel is required to be chronological.
	for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
		entries[left], entries[right] = entries[right], entries[left]
	}
	result, err := workexport.BuildExcel(month, projectName, entries, time.Now().In(b.service.Location()))
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	_, err = client.SendDocument(ctx, &bot.SendDocumentParams{ChatID: chatID, Document: &models.InputFileUpload{Filename: result.Filename, Data: bytes.NewReader(result.Data)}, Caption: "Готово: " + monthTitle(month), ReplyMarkup: mainMenu()})
	if err != nil {
		b.logger.Error("failed to send Excel", "error", err)
		b.sendError(ctx, client, chatID, errors.New("не удалось отправить Excel"))
	}
}

func (b *Bot) showProjects(ctx context.Context, client *bot.Bot, userID, chatID int64) {
	projects, err := b.service.ListProjects(ctx, false)
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	var out strings.Builder
	out.WriteString("📁 Проекты\n")
	rows := make([][]models.InlineKeyboardButton, 0, len(projects)+2)
	for _, p := range projects {
		status := "Активен"
		if !p.IsActive {
			status = "Неактивен"
		}
		fmt.Fprintf(&out, "\n%s\n%s\n%s\n", p.Name, money.FormatRate(p.HourlyRateKopecks), status)
		rows = append(rows, []models.InlineKeyboardButton{button(p.Name, fmt.Sprintf("project:view:%d", p.ID))})
	}
	if len(projects) == 0 {
		out.WriteString("\nПроектов пока нет.")
	}
	rows = append(rows, []models.InlineKeyboardButton{button("➕ Новый проект", "newproject")}, []models.InlineKeyboardButton{button("🏠 Меню", "menu")})
	b.sessions.clear(userID)
	b.send(ctx, client, chatID, out.String(), keyboard(rows...))
}
func (b *Bot) showProject(ctx context.Context, client *bot.Bot, chatID int64, p domain.Project) {
	status := "Активен"
	if !p.IsActive {
		status = "Неактивен"
	}
	rows := [][]models.InlineKeyboardButton{{button("✏️ Название", fmt.Sprintf("project:name:%d", p.ID)), button("💰 Ставка", fmt.Sprintf("project:rate:%d", p.ID))}}
	if p.IsActive {
		rows = append(rows, []models.InlineKeyboardButton{button("⛔ Деактивировать", fmt.Sprintf("project:deactivate:%d", p.ID))})
	}
	rows = append(rows, []models.InlineKeyboardButton{button("⬅️ Проекты", "projects")})
	b.send(ctx, client, chatID, fmt.Sprintf("%s\n%s\n%s", p.Name, money.FormatRate(p.HourlyRateKopecks), status), keyboard(rows...))
}
func (b *Bot) handleProjectCallback(ctx context.Context, client *bot.Bot, userID, chatID int64, parts []string) {
	if len(parts) < 3 {
		return
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return
	}
	switch parts[1] {
	case "view":
		p, err := b.service.GetProject(ctx, id)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.showProject(ctx, client, chatID, p)
	case "name":
		b.sessions.set(userID, session{Step: stepProjectEditName, ProjectID: id})
		b.send(ctx, client, chatID, "Новое название проекта?", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
	case "rate":
		b.sessions.set(userID, session{Step: stepProjectEditRate, ProjectID: id})
		b.send(ctx, client, chatID, "Новая ставка в рублях за час?", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
	case "deactivate":
		p, err := b.service.DeactivateProject(ctx, id)
		if err != nil {
			b.sendError(ctx, client, chatID, err)
			return
		}
		b.showProject(ctx, client, chatID, p)
	}
}

func (b *Bot) showPayments(ctx context.Context, client *bot.Bot, userID, chatID int64, month time.Time) {
	report, err := b.service.MonthlyReport(ctx, month)
	if err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	b.sessions.set(userID, session{Month: report.Month})
	var out strings.Builder
	fmt.Fprintf(&out, "💳 %s\n", monthTitle(report.Month))
	rows := make([][]models.InlineKeyboardButton, 0, len(report.Projects)+2)
	for _, p := range report.Projects {
		status := "🔴 Не оплачено"
		action := "paid"
		label := "✅ Отметить оплачено"
		if p.PaymentStatus == domain.PaymentPaid {
			status = "🟢 Оплачено"
			action = "pending"
			label = "↩️ Отметить неоплаченным"
			if p.PaidAt != nil {
				status += "\nДата оплаты: " + p.PaidAt.In(b.service.Location()).Format("02.01.2006 15:04")
			}
		}
		fmt.Fprintf(&out, "\n%s\n%s\n%s\n", p.ProjectName, money.FormatKopecks(p.AmountKopecks), status)
		rows = append(rows, []models.InlineKeyboardButton{button(p.ProjectName+": "+label, fmt.Sprintf("payments:set:%s:%d:%s", report.Month.Format("2006-01"), p.ProjectID, action))})
	}
	if len(report.Projects) == 0 {
		out.WriteString("\nЗа этот месяц работ нет.")
	}
	rows = append(rows, []models.InlineKeyboardButton{button("📅 Выбрать месяц", "payments:choose")}, []models.InlineKeyboardButton{button("🏠 Меню", "menu")})
	b.send(ctx, client, chatID, out.String(), keyboard(rows...))
}
func (b *Bot) handlePaymentCallback(ctx context.Context, client *bot.Bot, userID, chatID int64, parts []string) {
	if len(parts) < 2 {
		return
	}
	if parts[1] == "choose" {
		s := b.sessions.get(userID)
		s.Step = stepPaymentMonth
		b.sessions.set(userID, s)
		b.send(ctx, client, chatID, "Введите месяц в формате ГГГГ-ММ:", keyboard([]models.InlineKeyboardButton{button("❌ Отмена", "cancel")}))
		return
	}
	if parts[1] != "set" || len(parts) != 5 {
		return
	}
	month, err := service.ParseMonth(parts[2], b.service.Location())
	if err != nil {
		return
	}
	projectID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return
	}
	if err := b.service.SetPaymentStatus(ctx, projectID, month, parts[4]); err != nil {
		b.sendError(ctx, client, chatID, err)
		return
	}
	b.showPayments(ctx, client, userID, chatID, month)
}

func (b *Bot) send(ctx context.Context, client *bot.Bot, chatID int64, text string, markup models.ReplyMarkup) {
	if _, err := client.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: markup}); err != nil {
		b.logger.Error("failed to send Telegram message", "error", err)
	}
}
func (b *Bot) sendError(ctx context.Context, client *bot.Bot, chatID int64, err error) {
	b.logger.Error("Telegram operation failed", "error", err)
	b.send(ctx, client, chatID, "Не удалось выполнить действие. "+friendlyError(err), nil)
}
func friendlyError(err error) string {
	if errors.Is(err, pgx.ErrNoRows) {
		return "Запись не найдена."
	}
	msg := err.Error()
	if strings.Contains(msg, ":") {
		return "Попробуйте ещё раз."
	}
	return msg
}
