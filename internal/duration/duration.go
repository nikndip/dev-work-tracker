package duration

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const MaxSeconds int64 = 24 * 60 * 60

var (
	clockPattern   = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
	decimalPattern = regexp.MustCompile(`^(\d+)[\.,](\d+)\s*(?:ч|час|часа|часов)$`)
	unitsPattern   = regexp.MustCompile(`^(?:(\d+)\s*(?:ч|час|часа|часов)\s*)?(?:(\d+)\s*(?:м|мин|минута|минуты|минут)\s*)?$`)
)

func Parse(value string) (int64, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return 0, errors.New("empty duration")
	}
	if match := clockPattern.FindStringSubmatch(value); match != nil {
		hours, _ := strconv.ParseInt(match[1], 10, 64)
		minutes, _ := strconv.ParseInt(match[2], 10, 64)
		if minutes >= 60 {
			return 0, errors.New("minutes must be less than 60")
		}
		return validate(hours*3600 + minutes*60)
	}
	if match := decimalPattern.FindStringSubmatch(value); match != nil {
		whole, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || whole > MaxSeconds/3600 {
			return 0, errors.New("invalid decimal duration")
		}
		fraction, err := strconv.ParseInt(match[2], 10, 64)
		if err != nil || len(match[2]) > 6 {
			return 0, errors.New("invalid decimal duration")
		}
		scale := int64(1)
		for range len(match[2]) {
			scale *= 10
		}
		if fraction > math.MaxInt64/3600 {
			return 0, errors.New("invalid decimal duration")
		}
		numerator := fraction * 3600
		if numerator%scale != 0 {
			return 0, errors.New("duration must resolve to whole seconds")
		}
		return validate(whole*3600 + numerator/scale)
	}
	if match := unitsPattern.FindStringSubmatch(value); match != nil && (match[1] != "" || match[2] != "") {
		var hours, minutes int64
		if match[1] != "" {
			hours, _ = strconv.ParseInt(match[1], 10, 64)
		}
		if match[2] != "" {
			minutes, _ = strconv.ParseInt(match[2], 10, 64)
		}
		if hours > MaxSeconds/3600 || minutes > MaxSeconds/60 {
			return 0, errors.New("duration is too large")
		}
		return validate(hours*3600 + minutes*60)
	}
	return 0, errors.New("unrecognized duration")
}

func validate(seconds int64) (int64, error) {
	if seconds <= 0 || seconds > MaxSeconds {
		return 0, errors.New("duration must be between 1 second and 24 hours")
	}
	return seconds, nil
}

func Format(seconds int64) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%d ч %d мин", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%d ч", hours)
	default:
		return fmt.Sprintf("%d мин", minutes)
	}
}

func TotalMinutes(seconds int64) int64 {
	return seconds / 60
}

func FormatReport(seconds int64) string {
	minutes := TotalMinutes(seconds)
	if minutes == 0 {
		return "0 мин"
	}
	return fmt.Sprintf("%s (%d мин)", Format(seconds), minutes)
}
