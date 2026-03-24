package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/emarc09/fuelcheck/internal/auth"
	"github.com/emarc09/fuelcheck/internal/i18n"
)

const (
	claudeOAuthUsageURL = "https://api.anthropic.com/api/oauth/usage"
	claudeOrgsURL       = "https://claude.ai/api/organizations"
)

// claudeWindowKeys maps API keys to i18n translation keys.
var claudeWindowKeys = []struct {
	apiKey  string
	i18nKey string
}{
	{"five_hour", "window.5h"},
	{"seven_day", "window.weekly"},
	{"seven_day_sonnet", "window.weekly_sonnet"},
	{"seven_day_opus", "window.weekly_opus"},
}

// FetchClaudeUsage fetches usage data from Claude's API.
func FetchClaudeUsage() *ProviderResult {
	result := &ProviderResult{Provider: "Claude", OK: false}

	creds, webSession, err := auth.GetClaudeCredentials()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	if creds != nil && creds.Token != "" {
		oauthResult := fetchClaudeOAuth(creds)
		if oauthResult != nil {
			return oauthResult
		}
	}

	if webSession != nil {
		return fetchClaudeWeb(webSession)
	}

	if creds != nil {
		result.Error = i18n.T("err.claude.oauth_invalid")
		return result
	}

	result.Error = i18n.T("err.claude.no_creds_found")
	return result
}

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
		result.Error = i18n.Tf("err.create_request", err)
		return result
	}

	req.Header.Set("Authorization", "Bearer "+creds.Token)
	req.Header.Set("User-Agent", "claude-usage")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := httpClient.Do(req)
	if err != nil {
		result.Error = i18n.Tf("err.connection", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = i18n.Tf("err.read_response", err)
		return result
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		result.Error = i18n.T("err.too_many_requests")
		return result
	}

	if resp.StatusCode != http.StatusOK {
		result.Error = i18n.Tf("err.api_status", resp.StatusCode)
		return result
	}

	var usage map[string]json.RawMessage
	if err := json.Unmarshal(body, &usage); err != nil {
		result.Error = i18n.Tf("err.parse_json", err)
		return result
	}

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

func fetchClaudeWeb(ws *auth.ClaudeWebSession) *ProviderResult {
	result := &ProviderResult{Provider: "Claude", OK: false, Source: "web"}

	req, err := http.NewRequest("GET", claudeOrgsURL, nil)
	if err != nil {
		result.Error = i18n.Tf("err.create_request", err)
		return result
	}
	req.Header.Set("Cookie", "sessionKey="+ws.SessionKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "claude-usage")

	resp, err := httpClient.Do(req)
	if err != nil {
		result.Error = i18n.Tf("err.claude.orgs_error", err)
		return result
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = i18n.Tf("err.read_response", err)
		return result
	}
	if resp.StatusCode != http.StatusOK {
		result.Error = i18n.Tf("err.claude.orgs_status", resp.StatusCode)
		return result
	}

	var orgs []map[string]interface{}
	if err := json.Unmarshal(body, &orgs); err != nil || len(orgs) == 0 {
		result.Error = i18n.T("err.claude.no_orgs")
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
		result.Error = i18n.Tf("err.create_request", err)
		return result
	}
	req2.Header.Set("Cookie", "sessionKey="+ws.SessionKey)
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("User-Agent", "claude-usage")

	resp2, err := httpClient.Do(req2)
	if err != nil {
		result.Error = i18n.Tf("err.claude.usage_error", err)
		return result
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		result.Error = i18n.Tf("err.claude.usage_read_error", err)
		return result
	}
	if resp2.StatusCode != http.StatusOK {
		result.Error = i18n.Tf("err.claude.usage_status", resp2.StatusCode)
		return result
	}

	var usage map[string]json.RawMessage
	if err := json.Unmarshal(body2, &usage); err != nil {
		result.Error = i18n.Tf("err.claude.usage_parse", err)
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

	for _, wd := range claudeWindowKeys {
		raw, ok := usage[wd.apiKey]
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

		if usedPct == 0 && window.ResetsAt == "" {
			continue
		}

		remaining := max(0, int(100-usedPct+0.5))

		w := UsageWindow{
			Label:       i18n.T(wd.i18nKey),
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

// ParseISO8601 parses ISO 8601 timestamps.
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

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// CleanPlanName is exported for tests.
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
