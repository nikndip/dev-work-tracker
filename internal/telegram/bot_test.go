package telegram

import (
	"strings"
	"testing"
	"time"

	"dev-work-tracker/internal/domain"
	"dev-work-tracker/internal/service"

	"github.com/go-telegram/bot/models"
)

func TestAuthorizerAllowed(t *testing.T) {
	authorizer := Authorizer{AllowedUserID: 123456789}
	if !authorizer.Allowed(123456789) {
		t.Fatal("configured user must be allowed")
	}
	if authorizer.Allowed(1) {
		t.Fatal("unconfigured user must be denied")
	}
}

func TestYesterdayCallbackUpdatesDraftWithoutSaving(t *testing.T) {
	markup := dateKeyboard()
	if got := markup.InlineKeyboard[0][1].CallbackData; got != "new:datevalue:yesterday" {
		t.Fatalf("yesterday callback_data = %q", got)
	}

	location := time.FixedZone("Europe/Moscow", 3*60*60)
	yesterday := time.Date(2026, 9, 25, 0, 0, 0, 0, location)
	draft := session{Step: stepAddDescription, ProjectID: 7, Description: "Работа"}
	updated := selectDraftDate(draft, yesterday)
	if !updated.WorkDate.Equal(yesterday) {
		t.Fatalf("draft date = %v, want %v", updated.WorkDate, yesterday)
	}
	if updated.Step != stepAddDuration {
		t.Fatalf("draft step = %q, want %q", updated.Step, stepAddDuration)
	}
	if updated.EntryID != 0 {
		t.Fatal("date callback must not create a WorkEntry")
	}
	repeated := selectDraftDate(updated, yesterday)
	if repeated.EntryID != 0 || !repeated.WorkDate.Equal(yesterday) || repeated.Step != stepAddDuration {
		t.Fatalf("repeated callback changed draft unexpectedly: %+v", repeated)
	}
}

func TestReportTextIncludesProjectAndMonthlyAmounts(t *testing.T) {
	report := domain.MonthlyReport{
		Month: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Projects: []domain.ProjectReport{{
			ProjectName: "ProjectHub", EntryCount: 13, DurationSeconds: 41700, AmountKopecks: 2316668,
		}},
		EntryCount: 13, DurationSeconds: 41700, AmountKopecks: 2316668,
	}
	text := reportText(report)
	for _, want := range []string{
		"ProjectHub", "Работ: 13", "Фактическое время: 11 ч 35 мин (695 мин)", "Сумма: 23 166,68 ₽",
		"Итого:\n11 ч 35 мин (695 мин)\n23 166,68 ₽",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report does not contain %q:\n%s", want, text)
		}
	}
}

func TestEmptyReportIncludesZeroTotals(t *testing.T) {
	text := reportText(domain.MonthlyReport{Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)})
	for _, want := range []string{"Октябрь 2026", "За этот месяц работ пока нет.", "Работ: 0", "Фактическое время: 0 мин", "Сумма: 0 ₽"} {
		if !strings.Contains(text, want) {
			t.Fatalf("empty report does not contain %q:\n%s", want, text)
		}
	}
}

func TestReportNavigation(t *testing.T) {
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 1, 0, 10, 0, 0, location)
	current, _ := service.MonthBounds(now, location)

	september := time.Date(2026, 9, 1, 0, 0, 0, 0, location)
	markup := reportKeyboard(september, current)
	if !hasCallback(markup, "report:prev:2026-09") || !hasCallback(markup, "report:next:2026-09") {
		t.Fatalf("past-month navigation is incomplete: %+v", markup.InlineKeyboard)
	}

	markup = reportKeyboard(current, current)
	if hasCallback(markup, "report:next:2026-10") {
		t.Fatal("current month must not have a next-month button")
	}

	next, ok := nextReportMonth(september, current)
	if !ok || next.Format("2006-01") != "2026-10" {
		t.Fatalf("September next month = %s, %v; want 2026-10, true", next.Format("2006-01"), ok)
	}
	if _, ok := nextReportMonth(current, current); ok {
		t.Fatal("navigation from the current month into the future must be rejected")
	}
	if previous := previousReportMonth(september); previous.Format("2006-01") != "2026-08" {
		t.Fatalf("September previous month = %s; want 2026-08", previous.Format("2006-01"))
	}

	december := time.Date(2026, 12, 1, 0, 0, 0, 0, location)
	january := time.Date(2027, 1, 1, 0, 0, 0, 0, location)
	if next, ok := nextReportMonth(december, january); !ok || next.Format("2006-01") != "2027-01" {
		t.Fatalf("December next month = %s, %v; want 2027-01, true", next.Format("2006-01"), ok)
	}
	if previous := previousReportMonth(january); previous.Format("2006-01") != "2026-12" {
		t.Fatalf("January previous month = %s; want 2026-12", previous.Format("2006-01"))
	}
}

func hasCallback(markup *models.InlineKeyboardMarkup, callback string) bool {
	for _, row := range markup.InlineKeyboard {
		for _, item := range row {
			if item.CallbackData == callback {
				return true
			}
		}
	}
	return false
}

func TestUpdateUserID(t *testing.T) {
	tests := []struct {
		name   string
		update *models.Update
		wantID int64
		wantOK bool
	}{
		{
			name:   "message",
			update: &models.Update{Message: &models.Message{From: &models.User{ID: 123456789}}},
			wantID: 123456789,
			wantOK: true,
		},
		{
			name:   "callback query",
			update: &models.Update{CallbackQuery: &models.CallbackQuery{From: models.User{ID: 123456789}}},
			wantID: 123456789,
			wantOK: true,
		},
		{name: "unsupported update", update: &models.Update{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := updateUserID(tt.update)
			if gotID != tt.wantID || gotOK != tt.wantOK {
				t.Fatalf("updateUserID() = (%d, %v), want (%d, %v)", gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}
