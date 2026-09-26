package telegram

import (
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
)

func TestAuthorizerAllowed(t *testing.T) {
	authorizer := Authorizer{AllowedUserID: 624740467}
	if !authorizer.Allowed(624740467) {
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

func TestUpdateUserID(t *testing.T) {
	tests := []struct {
		name   string
		update *models.Update
		wantID int64
		wantOK bool
	}{
		{
			name:   "message",
			update: &models.Update{Message: &models.Message{From: &models.User{ID: 624740467}}},
			wantID: 624740467,
			wantOK: true,
		},
		{
			name:   "callback query",
			update: &models.Update{CallbackQuery: &models.CallbackQuery{From: models.User{ID: 624740467}}},
			wantID: 624740467,
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
