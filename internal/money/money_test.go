package money

import "testing"

func TestCalculateAmount(t *testing.T) {
	tests := []struct {
		name    string
		seconds int64
		want    int64
	}{
		{"1 minute", 60, 3333},
		{"30 minutes", 1800, 100000},
		{"45 minutes", 2700, 150000},
		{"1 hour", 3600, 200000},
		{"1 hour 20 minutes", 4800, 266667},
		{"1 hour 30 minutes", 5400, 300000},
		{"2 hours", 7200, 400000},
		{"2 hours 30 minutes", 9000, 500000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateAmount(200000, tt.seconds)
			if err != nil || got != tt.want {
				t.Fatalf("CalculateAmount() = %d, %v; want %d", got, err, tt.want)
			}
		})
	}
}

func TestParseAndFormatRubles(t *testing.T) {
	for input, want := range map[string]int64{"2000": 200000, "2500": 250000, "2000.50": 200050, "2000,50": 200050} {
		got, err := ParseRubles(input)
		if err != nil || got != want {
			t.Errorf("ParseRubles(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
	if got := FormatKopecks(266667); got != "2 666,67 ₽" {
		t.Fatalf("FormatKopecks() = %q", got)
	}
}
