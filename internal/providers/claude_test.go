package providers

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseISO8601(t *testing.T) {
	tests := []struct {
		input string
		want  bool // true if should parse successfully
	}{
		{"2026-03-24T18:00:01.133952+00:00", true},
		{"2026-03-24T18:00:00Z", true},
		{"2026-03-24T18:00:00", true},
		{"", false},
		{"not-a-date", false},
	}

	for _, tt := range tests {
		result := ParseISO8601(tt.input)
		if tt.want && result == nil {
			t.Errorf("ParseISO8601(%q) = nil, want non-nil", tt.input)
		}
		if !tt.want && result != nil {
			t.Errorf("ParseISO8601(%q) = %v, want nil", tt.input, result)
		}
	}
}

func TestCleanPlanName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"stripe_subscription", "Pro"},
		{"pro", "Pro"},
		{"Pro", "Pro"},
		{"free", "Free"},
		{"team", "Team"},
		{"enterprise", "Enterprise"},
		{"max", "Max"},
		{"custom_plan", "Custom_plan"},
		{"", ""},
	}

	for _, tt := range tests {
		got := CleanPlanName(tt.input)
		if got != tt.want {
			t.Errorf("CleanPlanName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseClaudeWindows(t *testing.T) {
	// Simulate API response with mixed utilization formats.
	usageJSON := `{
		"five_hour": {"utilization": 45, "resets_at": "2026-03-24T18:00:00Z"},
		"seven_day": {"utilization": 0.1, "resets_at": "2026-03-30T00:00:00Z"},
		"seven_day_sonnet": {"utilization": 0},
		"seven_day_opus": {"utilization": 30}
	}`

	var usage map[string]json.RawMessage
	if err := json.Unmarshal([]byte(usageJSON), &usage); err != nil {
		t.Fatal(err)
	}

	windows := parseClaudeWindows(usage)

	// five_hour: 45% used -> 55% remaining
	// seven_day: 0.1 (fraction) -> 10% used -> 90% remaining
	// seven_day_sonnet: 0% used, no resets_at -> SKIPPED
	// seven_day_opus: 30% used, no resets_at but non-zero -> 70% remaining

	if len(windows) != 3 {
		t.Fatalf("got %d windows, want 3", len(windows))
	}

	if windows[0].Remaining != 55 {
		t.Errorf("five_hour remaining = %d, want 55", windows[0].Remaining)
	}
	if windows[0].ResetsAt == nil {
		t.Error("five_hour ResetsAt should not be nil")
	}

	if windows[1].Remaining != 90 {
		t.Errorf("seven_day remaining = %d, want 90", windows[1].Remaining)
	}

	if windows[2].Label != "Límite semanal Opus" {
		t.Errorf("third window label = %q, want Límite semanal Opus", windows[2].Label)
	}
	if windows[2].Remaining != 70 {
		t.Errorf("seven_day_opus remaining = %d, want 70", windows[2].Remaining)
	}
}

func TestParseClaudeWindowsSkipsZeroNoReset(t *testing.T) {
	usageJSON := `{
		"five_hour": {"utilization": 10, "resets_at": "2026-03-24T18:00:00Z"},
		"seven_day_sonnet": {"utilization": 0},
		"seven_day_opus": {"utilization": 0}
	}`

	var usage map[string]json.RawMessage
	json.Unmarshal([]byte(usageJSON), &usage)

	windows := parseClaudeWindows(usage)

	if len(windows) != 1 {
		t.Fatalf("got %d windows, want 1 (sonnet/opus should be skipped)", len(windows))
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct{ input, want string }{
		{"pro", "Pro"},
		{"", ""},
		{"A", "A"},
		{"hello world", "Hello world"},
	}
	for _, tt := range tests {
		if got := capitalize(tt.input); got != tt.want {
			t.Errorf("capitalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseISO8601LocalTime(t *testing.T) {
	result := ParseISO8601("2026-03-24T18:00:00Z")
	if result == nil {
		t.Fatal("expected non-nil")
	}
	// Should be converted to local time.
	if result.Location() != time.Now().Location() {
		t.Errorf("expected local timezone, got %v", result.Location())
	}
}
