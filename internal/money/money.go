package money

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const maxDurationSeconds int64 = 24 * 60 * 60

func CalculateAmount(hourlyRateKopecks, durationSeconds int64) (int64, error) {
	if hourlyRateKopecks <= 0 {
		return 0, errors.New("hourly rate must be positive")
	}
	if durationSeconds <= 0 || durationSeconds > maxDurationSeconds {
		return 0, errors.New("duration must be between 1 second and 24 hours")
	}
	if hourlyRateKopecks > (math.MaxInt64-1800)/durationSeconds {
		return 0, errors.New("amount overflow")
	}
	return (hourlyRateKopecks*durationSeconds + 1800) / 3600, nil
}

func ParseRubles(value string) (int64, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), " ", "")
	value = strings.ReplaceAll(value, ",", ".")
	if value == "" || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		return 0, errors.New("invalid amount")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid amount")
	}
	rubles, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || rubles <= 0 || rubles > math.MaxInt64/100 {
		return 0, errors.New("invalid amount")
	}
	var kopecks int64
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, errors.New("use no more than two decimal places")
		}
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		kopecks, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("invalid amount")
		}
	}
	if rubles > (math.MaxInt64-kopecks)/100 {
		return 0, errors.New("amount overflow")
	}
	return rubles*100 + kopecks, nil
}

func FormatKopecks(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	rubles := groupThousands(value / 100)
	if value%100 == 0 {
		return fmt.Sprintf("%s%s ₽", sign, rubles)
	}
	return fmt.Sprintf("%s%s,%02d ₽", sign, rubles, value%100)
}

func FormatRate(value int64) string {
	return FormatKopecks(value) + "/ч"
}

func groupThousands(value int64) string {
	digits := strconv.FormatInt(value, 10)
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + " " + digits[i:]
	}
	return digits
}
