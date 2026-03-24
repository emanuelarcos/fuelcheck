package i18n

import (
	"os"
	"testing"
)

func TestDetectEnglishDefault(t *testing.T) {
	os.Unsetenv("FUELCHECK_LANG")
	os.Unsetenv("LC_ALL")
	os.Unsetenv("LANG")
	os.Unsetenv("LANGUAGE")

	lang := Detect()
	if lang != EN {
		t.Errorf("Detect() = %q, want %q", lang, EN)
	}
}

func TestDetectSpanishFromLang(t *testing.T) {
	os.Unsetenv("FUELCHECK_LANG")
	os.Unsetenv("LC_ALL")
	os.Setenv("LANG", "es_AR.UTF-8")
	defer os.Unsetenv("LANG")

	lang := Detect()
	if lang != ES {
		t.Errorf("Detect() = %q, want %q", lang, ES)
	}
}

func TestDetectFuelcheckLangOverride(t *testing.T) {
	os.Setenv("FUELCHECK_LANG", "es")
	os.Setenv("LANG", "en_US.UTF-8")
	defer os.Unsetenv("FUELCHECK_LANG")
	defer os.Unsetenv("LANG")

	lang := Detect()
	if lang != ES {
		t.Errorf("Detect() = %q, want %q (FUELCHECK_LANG should override)", lang, ES)
	}
}

func TestSet(t *testing.T) {
	Set("es")
	if Current() != ES {
		t.Errorf("Current() = %q after Set(\"es\"), want %q", Current(), ES)
	}
	Set("en")
	if Current() != EN {
		t.Errorf("Current() = %q after Set(\"en\"), want %q", Current(), EN)
	}
	Set("invalid")
	if Current() != EN {
		t.Errorf("Current() = %q after Set(\"invalid\"), want %q (fallback)", Current(), EN)
	}
}

func TestT(t *testing.T) {
	Set("en")
	if got := T("window.5h"); got != "5-hour usage limit" {
		t.Errorf("T(\"window.5h\") en = %q", got)
	}

	Set("es")
	if got := T("window.5h"); got != "Límite de uso de 5 horas" {
		t.Errorf("T(\"window.5h\") es = %q", got)
	}
}

func TestTFallbackToKey(t *testing.T) {
	Set("en")
	if got := T("nonexistent.key"); got != "nonexistent.key" {
		t.Errorf("T(\"nonexistent.key\") = %q, want the key itself", got)
	}
}

func TestTf(t *testing.T) {
	Set("en")
	got := Tf("ui.remaining", 75)
	if got != "75% remaining" {
		t.Errorf("Tf(\"ui.remaining\", 75) = %q", got)
	}

	Set("es")
	got = Tf("ui.remaining", 75)
	if got != "75% restante" {
		t.Errorf("Tf(\"ui.remaining\", 75) = %q", got)
	}
}

func TestMonthName(t *testing.T) {
	Set("en")
	if got := MonthName(3); got != "Mar" {
		t.Errorf("MonthName(3) en = %q", got)
	}

	Set("es")
	if got := MonthName(3); got != "mar" {
		t.Errorf("MonthName(3) es = %q", got)
	}

	if got := MonthName(0); got != "" {
		t.Errorf("MonthName(0) = %q, want empty", got)
	}
	if got := MonthName(13); got != "" {
		t.Errorf("MonthName(13) = %q, want empty", got)
	}
}
