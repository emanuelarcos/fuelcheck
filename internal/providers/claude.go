package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/emarc09/fuelcheck/internal/auth"
)

const (
	claudeOAuthUsageURL = "https://api.anthropic.com/api/oauth/usage"
	claudeOrgsURL       = "https://claude.ai/api/organizations"
)

// claudeWindowDefs defines the usage windows to parse from the API response.
var claudeWindowDefs = []struct {
	key   string
	label string
}{
	{"five_hour", "Límite de uso de 5 horas"},
	{"seven_day", "Límite de uso semanal"},
	{"seven_day_sonnet", "Límite semanal Sonnet"},
	{"seven_day_opus", "Límite semanal Opus"},
}

// FetchClaudeUsage fetches usage data from Claude's API.
func FetchClaudeUsage() *ProviderResult {
	result := &ProviderResult{Provider: "Claude", OK: false}

	creds, webSession, err := auth.GetClaudeCredentials()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Try OAuth path first.
	if creds != nil && creds.Token != "" {
		oauthResult := fetchClaudeOAuth(creds)
		if oauthResult != nil {
			return oauthResult
		}
		// OAuth returned 403 — fall through to web session if available.
	}

	// Web session fallback.
	if webSession != nil {
		return fetchClaudeWeb(webSession)
	}

	if creds != nil {
		result.Error = "OAuth token inválido y no hay sesión web disponible"
		return result
	}

	result.Error = "no se encontraron credenciales de Claude"
	return result
}

// fetchClaudeOAuth fetches usage via the OAuth API.
func fetchClaudeOAuth(creds *auth.ClaudeCredentials) *ProviderResult {
	result := &ProviderResult{
		Provider: "Claude",
		OK:       false,
		Source:   creds.Source,
		Plan:     creds.SubscriptionType,
		Tier:     creds.RateLimitTier,
		Email:    creds.Email,
	}

	req, err := http.NewRequest("GET", claudeOAuthUsageURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("error al crear request: %v", err)
		return result
	}

	req.Header.Set("Authorization", "Bearer "+creds.Token)
	req.Header.Set("User-Agent", "claude-usage")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("error de conexión: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil // signal to fall through to web session
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("error al leer respuesta: %v", err)
		return result
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		result.Error = "Too many requests — esperá unos minutos e intentá de nuevo"
		return result
	}

	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("API respondió con status %d", resp.StatusCode)
		return result
	}

	var usage map[string]json.RawMessage
	if err := json.Unmarshal(body, &usage); err != nil {
		result.Error = fmt.Sprintf("error al parsear JSON: %v", err)
		return result
	}

	// Store raw JSON for --json mode.
	var rawJSON interface{}
	_ = json.Unmarshal(body, &rawJSON)
	result.RawJSON = map[string]interface{}{
		"usage":   rawJSON,
		"account": map[string]string{"subscriptionType": creds.SubscriptionType, "rateLimitTier": creds.RateLimitTier},
		"source":  creds.Source,
	}

	result.Windows = parseClaudeWindows(usage)

	if result.Plan != "" {
		result.Plan = cleanPlanName(result.Plan)
	}

	result.OK = true
	return result
}

// fetchClaudeWeb fetches usage via the web session API.
func fetchClaudeWeb(ws *auth.ClaudeWebSession) *ProviderResult {
	result := &ProviderResult{Provider: "Claude", OK: false, Source: "web"}

	req, err := http.NewRequest("GET", claudeOrgsURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("error al crear request: %v", err)
		return result
	}
	req.Header.Set("Cookie", "sessionKey="+ws.SessionKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "claude-usage")

	resp, err := httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("error al obtener organizaciones: %v", err)
		return result
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("error al leer respuesta: %v", err)
		return result
	}
	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("error al obtener organizaciones: status %d", resp.StatusCode)
		return result
	}

	var orgs []map[string]interface{}
	if err := json.Unmarshal(body, &orgs); err != nil || len(orgs) == 0 {
		result.Error = "no se encontraron organizaciones"
		return result
	}

	org := orgs[0]
	orgUUID, _ := org["uuid"].(string)

	for _, key := range []string{"name", "display_name", "uuid"} {
		if v, ok := org[key].(string); ok && v != "" {
			result.Plan = capitalize(v)
			break
		}
	}

	usageURL := fmt.Sprintf("https://claude.ai/api/organizations/%s/usage", orgUUID)
	req2, err := http.NewRequest("GET", usageURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("error al crear request: %v", err)
		return result
	}
	req2.Header.Set("Cookie", "sessionKey="+ws.SessionKey)
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("User-Agent", "claude-usage")

	resp2, err := httpClient.Do(req2)
	if err != nil {
		result.Error = fmt.Sprintf("error al obtener uso: %v", err)
		return result
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		result.Error = fmt.Sprintf("error al leer respuesta de uso: %v", err)
		return result
	}
	if resp2.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("error al obtener uso: status %d", resp2.StatusCode)
		return result
	}

	var usage map[string]json.RawMessage
	if err := json.Unmarshal(body2, &usage); err != nil {
		result.Error = fmt.Sprintf("error al parsear uso: %v", err)
		return result
	}

	var rawUsage interface{}
	_ = json.Unmarshal(body2, &rawUsage)
	result.RawJSON = map[string]interface{}{
		"usage":  rawUsage,
		"org":    org,
		"source": "web",
	}

	result.Windows = parseClaudeWindows(usage)
	result.OK = true
	return result
}

// parseClaudeWindows extracts usage windows from a Claude API response.
func parseClaudeWindows(usage map[string]json.RawMessage) []UsageWindow {
	var windows []UsageWindow

	for _, wd := range claudeWindowDefs {
		raw, ok := usage[wd.key]
		if !ok {
			continue
		}

		var window struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    string  `json:"resets_at"`
		}
		if err := json.Unmarshal(raw, &window); err != nil {
			continue
		}

		usedPct := window.Utilization
		if usedPct <= 1.0 {
			usedPct *= 100
		}

		// Skip sub-model windows with zero usage — they're noise.
		if usedPct == 0 && window.ResetsAt == "" {
			continue
		}

		remaining := max(0, int(100-usedPct+0.5))

		w := UsageWindow{
			Label:       wd.label,
			UsedPercent: usedPct,
			Remaining:   remaining,
		}

		if window.ResetsAt != "" {
			if t := ParseISO8601(window.ResetsAt); t != nil {
				w.ResetsAt = t
			}
		}

		windows = append(windows, w)
	}

	return windows
}

// ParseISO8601 parses ISO 8601 timestamps (with or without trailing Z).
func ParseISO8601(s string) *time.Time {
	s = strings.TrimSpace(s)
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			local := t.Local()
			return &local
		}
	}
	return nil
}

// capitalize capitalizes the first letter of a string.
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// CleanPlanName converts raw subscription type strings to display names.
func CleanPlanName(s string) string {
	return cleanPlanName(s)
}

func cleanPlanName(s string) string {
	planMap := map[string]string{
		"stripe_subscription": "Pro",
		"pro":                 "Pro",
		"free":                "Free",
		"team":                "Team",
		"enterprise":          "Enterprise",
		"max":                 "Max",
	}
	lower := strings.ToLower(s)
	if name, ok := planMap[lower]; ok {
		return name
	}
	return capitalize(s)
}
