package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/emarc09/fuelcheck/internal/i18n"
	"github.com/emarc09/fuelcheck/internal/providers"
)

// Colors.
var (
	colorGreen  = lipgloss.Color("#04B575")
	colorYellow = lipgloss.Color("#F8D948")
	colorRed    = lipgloss.Color("#F653A6")
	colorCyan   = lipgloss.Color("#00BFFF")
	colorDim    = lipgloss.Color("#666666")
	colorWhite  = lipgloss.Color("#FFFFFF")
)

// Styles.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorCyan)

	valueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	sectionLabelStyle = lipgloss.NewStyle().
				Foreground(colorWhite)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(1, 2)
)

const barWidth = 22

// Banner returns the styled fuelcheck banner string.
func Banner() string {
	return renderBanner()
}

// renderBanner creates the styled fuelcheck banner with ASCII art flame.
func renderBanner() string {
	flameTopStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Bold(true)
	flameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B35")).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	tagStyle := dimStyle

	line1 := flameTopStyle.Render("  (  ") + nameStyle.Render("┏━╸╻ ╻┏━╸╻  ┏━╸╻ ╻┏━╸┏━╸╻┏")
	line2 := flameTopStyle.Render(" )\\) ") + nameStyle.Render("┣╸ ┃ ┃┣╸ ┃  ┃  ┣━┫┣╸ ┃  ┣┻┓")
	line3 := flameStyle.Render("((_) ") + nameStyle.Render("╹  ┗━┛┗━╸┗━╸┗━╸╹ ╹┗━╸┗━╸╹ ╹")

	tagline := tagStyle.Render("            " + i18n.T("ui.tagline"))

	return line1 + "\n" + line2 + "\n" + line3 + "\n" + tagline
}

// Render renders all provider results as styled terminal output.
func Render(results []*providers.ProviderResult) string {
	var sections []string

	sections = append(sections, renderBanner())

	for _, r := range results {
		card := renderProvider(r)
		if card != "" {
			sections = append(sections, card)
		}
	}

	return strings.Join(sections, "\n\n")
}

func renderProvider(r *providers.ProviderResult) string {
	title := titleStyle.Render(r.Provider)

	if !r.OK {
		errMsg := errorStyle.Render("Error: " + r.Error)
		content := title + "\n" + errMsg
		return cardStyle.Render(content)
	}

	var lines []string
	lines = append(lines, title)

	meta := renderMetadata(r)
	if meta != "" {
		lines = append(lines, meta)
	}

	if r.Hint != "" {
		lines = append(lines, hintStyle.Render(r.Hint))
	}

	for _, w := range r.Windows {
		lines = append(lines, "")
		lines = append(lines, sectionLabelStyle.Render(w.Label+":"))
		lines = append(lines, "")
		lines = append(lines, renderBar(w.Remaining))
		if w.ResetsAt != nil {
			lines = append(lines, dimStyle.Render(i18n.Tf("ui.resets_at", formatTime(w.ResetsAt))))
		}
	}

	for _, m := range r.Models {
		remaining := int(m.RemainingPercent + 0.5)
		lines = append(lines, "")
		lines = append(lines, sectionLabelStyle.Render(m.TierName+":"))
		lines = append(lines, "")
		lines = append(lines, renderBar(remaining))
		if m.ResetsAt != nil {
			lines = append(lines, dimStyle.Render(i18n.Tf("ui.resets_at", formatTime(m.ResetsAt))))
		}
	}

	return cardStyle.Render(strings.Join(lines, "\n"))
}

func renderMetadata(r *providers.ProviderResult) string {
	var parts []string

	switch r.Provider {
	case "Claude":
		if r.Plan != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.plan"))+valueStyle.Render(r.Plan))
		}
		if r.Email != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.email"))+valueStyle.Render(r.Email))
		}
		if r.Tier != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.tier"))+valueStyle.Render(r.Tier))
		}
		if r.Source != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.source"))+valueStyle.Render(r.Source))
		}
	case "Codex":
		if r.PlanType != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.plan"))+valueStyle.Render(r.PlanType))
		}
		if r.Email != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.email"))+valueStyle.Render(r.Email))
		}
	case "Gemini":
		if r.GeminiTier != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.tier"))+valueStyle.Render(r.GeminiTier))
		}
		if r.Email != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.email"))+valueStyle.Render(r.Email))
		}
		if r.AuthMethod != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.auth"))+valueStyle.Render(r.AuthMethod))
		}
		if r.TokenRefreshed {
			parts = append(parts, dimStyle.Render(i18n.T("ui.token_refreshed")))
		}
	case "Antigravity":
		if r.PlanType != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.plan"))+valueStyle.Render(r.PlanType))
		}
		if r.Email != "" {
			parts = append(parts, labelStyle.Render(i18n.T("ui.email"))+valueStyle.Render(r.Email))
		}
	}

	return strings.Join(parts, "\n")
}

func renderBar(remainingPct int) string {
	if remainingPct < 0 {
		remainingPct = 0
	}
	if remainingPct > 100 {
		remainingPct = 100
	}

	filled := int(float64(remainingPct) / 100.0 * float64(barWidth))
	empty := barWidth - filled

	var accent lipgloss.Color
	switch {
	case remainingPct >= 70:
		accent = colorGreen
	case remainingPct >= 35:
		accent = colorYellow
	default:
		accent = colorRed
	}

	barFilled := lipgloss.NewStyle().Foreground(accent).Render(strings.Repeat("█", filled))
	barEmpty := dimStyle.Render(strings.Repeat("░", empty))
	pctText := lipgloss.NewStyle().Bold(true).Foreground(accent).Render(" " + i18n.Tf("ui.remaining", remainingPct))

	return barFilled + barEmpty + pctText
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}

	now := time.Now()

	hour := t.Hour()
	minute := t.Minute()
	ampm := i18n.T("time.am")
	if hour >= 12 {
		ampm = i18n.T("time.pm")
	}
	if hour > 12 {
		hour -= 12
	}
	if hour == 0 {
		hour = 12
	}

	timeStr := fmt.Sprintf("%d:%02d %s", hour, minute, ampm)

	if t.Year() == now.Year() && t.Month() == now.Month() && t.Day() == now.Day() {
		return timeStr
	}

	month := i18n.MonthName(int(t.Month()))
	if i18n.Current() == i18n.ES {
		return fmt.Sprintf("%d %s %d %s", t.Day(), month, t.Year(), timeStr)
	}
	return fmt.Sprintf("%s %d, %d %s", month, t.Day(), t.Year(), timeStr)
}
