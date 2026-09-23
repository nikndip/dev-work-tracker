package telegram

import (
	"testing"

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
