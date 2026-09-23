package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultHTTPAddress = ":8080"

type Config struct {
	TelegramBotToken         string
	TelegramAllowedUserID    int64
	DatabaseURL              string
	DefaultTimezone          *time.Location
	DefaultHourlyRateKopecks int64
	HTTPAddress              string
}

func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	required := func(name string) (string, error) {
		value, ok := lookup(name)
		value = strings.TrimSpace(value)
		if !ok || value == "" {
			return "", fmt.Errorf("environment variable %s is required", name)
		}
		return value, nil
	}
	token, err := required("TELEGRAM_BOT_TOKEN")
	if err != nil {
		return Config{}, err
	}
	allowedRaw, err := required("TELEGRAM_ALLOWED_USER_ID")
	if err != nil {
		return Config{}, err
	}
	allowedID, err := strconv.ParseInt(allowedRaw, 10, 64)
	if err != nil || allowedID <= 0 {
		return Config{}, errors.New("TELEGRAM_ALLOWED_USER_ID must be a positive integer")
	}
	databaseURL, err := required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	timezoneName, err := required("DEFAULT_TIMEZONE")
	if err != nil {
		return Config{}, err
	}
	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		return Config{}, fmt.Errorf("invalid DEFAULT_TIMEZONE: %w", err)
	}
	rateRaw, err := required("DEFAULT_HOURLY_RATE_KOPECKS")
	if err != nil {
		return Config{}, err
	}
	rate, err := parsePositiveKopecks(rateRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid DEFAULT_HOURLY_RATE_KOPECKS: %w", err)
	}
	httpAddress := defaultHTTPAddress
	if value, ok := lookup("HTTP_ADDRESS"); ok && strings.TrimSpace(value) != "" {
		httpAddress = strings.TrimSpace(value)
	}
	return Config{token, allowedID, databaseURL, location, rate, httpAddress}, nil
}

func parsePositiveKopecks(value string) (int64, error) {
	kopecks, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errors.New("must be an integer number of kopecks")
	}
	if kopecks <= 0 {
		return 0, errors.New("must be greater than zero")
	}
	return kopecks, nil
}
