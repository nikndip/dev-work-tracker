package duration

import "testing"

func TestParse(t *testing.T) {
	valid := map[string]int64{
		"1 час 30 мин": 5400, "1 час 30 минут": 5400, "1ч 30м": 5400,
		"1 ч 30 мин": 5400, "90 мин": 5400, "90 минут": 5400, "90м": 5400,
		"1:30": 5400, "1.5 часа": 5400, "1,5 часа": 5400, "30 мин": 1800,
		"45 минут": 2700, "2 часа": 7200, "2 час": 7200, "2ч": 7200, "2:15": 8100, "135 мин": 8100,
	}
	for input, want := range valid {
		got, err := Parse(input)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
	for _, input := range []string{"", "abc", "130", "0 мин", "-30 мин", "1:90"} {
		if got, err := Parse(input); err == nil {
			t.Errorf("Parse(%q) = %d, want error", input, got)
		}
	}
}

func TestFormat(t *testing.T) {
	for seconds, want := range map[int64]string{5400: "1 ч 30 мин", 3600: "1 ч", 2700: "45 мин", 11400: "3 ч 10 мин"} {
		if got := Format(seconds); got != want {
			t.Errorf("Format(%d) = %q; want %q", seconds, got, want)
		}
	}
}
