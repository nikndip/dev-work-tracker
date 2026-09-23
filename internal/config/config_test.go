package config

import (
	"strings"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	cfg, err := load(mapLookup(validEnvironment()))
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if cfg.TelegramAllowedUserID != 624740467 || cfg.DefaultHourlyRateKopecks != 200000 {
		t.Fatalf("unexpected numeric config: %+v", cfg)
	}
	if cfg.DefaultTimezone.String() != "Europe/Moscow" || cfg.HTTPAddress != ":8080" {
		t.Fatalf("unexpected location/address: %s %s", cfg.DefaultTimezone, cfg.HTTPAddress)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	tests := []struct{ name, key, value, wantErr string }{
		{"missing token", "TELEGRAM_BOT_TOKEN", "", "TELEGRAM_BOT_TOKEN"},
		{"invalid user ID", "TELEGRAM_ALLOWED_USER_ID", "nope", "positive integer"},
		{"invalid timezone", "DEFAULT_TIMEZONE", "Mars/Olympus", "DEFAULT_TIMEZONE"},
		{"fractional rate", "DEFAULT_HOURLY_RATE_KOPECKS", "2000.50", "integer"},
		{"negative rate", "DEFAULT_HOURLY_RATE_KOPECKS", "-1", "greater than zero"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnvironment()
			env[tt.key] = tt.value
			_, err := load(mapLookup(env))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("load() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestParsePositiveKopecks(t *testing.T) {
	got, err := parsePositiveKopecks("200000")
	if err != nil || got != 200000 {
		t.Fatalf("parsePositiveKopecks() = %d, %v", got, err)
	}
}

func validEnvironment() map[string]string {
	return map[string]string{
		"TELEGRAM_BOT_TOKEN": "123456:test-token", "TELEGRAM_ALLOWED_USER_ID": "624740467",
		"DATABASE_URL": "postgres://tracker:password@localhost/tracker", "DEFAULT_TIMEZONE": "Europe/Moscow",
		"DEFAULT_HOURLY_RATE_KOPECKS": "200000",
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}
