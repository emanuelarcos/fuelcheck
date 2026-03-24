package providers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/emarc09/fuelcheck/internal/auth"
	"github.com/emarc09/fuelcheck/internal/i18n"
)

const (
	codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"
)

// FetchCodexUsage fetches usage data from the Codex/ChatGPT API.
func FetchCodexUsage() *ProviderResult {
	result := &ProviderResult{Provider: "Codex", OK: false}

	creds, err := auth.GetCodexCredentials()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Email = creds.Email

	body, statusCode, err := doCodexRequest(creds)
	if err != nil {
		result.Error = i18n.Tf("err.connection", err)
		return result
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		if refreshErr := creds.RefreshToken(); refreshErr != nil {
			result.Error = i18n.Tf("err.codex.token_expired", refreshErr)
			return result
		}

		body, statusCode, err = doCodexRequest(creds)
		if err != nil {
			result.Error = i18n.Tf("err.connection_retry", err)
			return result
		}
	}

	if statusCode != http.StatusOK {
		result.Error = i18n.Tf("err.api_status", statusCode)
		return result
	}

	var usage codexUsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		result.Error = i18n.Tf("err.parse_json", err)
		return result
	}

	var rawJSON interface{}
	_ = json.Unmarshal(body, &rawJSON)
	result.RawJSON = rawJSON

	result.PlanType = capitalize(usage.PlanType)
	result.OK = true

	if usage.RateLimit.PrimaryWindow.UsedPercent > 0 || usage.RateLimit.PrimaryWindow.ResetAt > 0 {
		remaining := max(0, int(100-usage.RateLimit.PrimaryWindow.UsedPercent+0.5))
		w := UsageWindow{
			Label:       i18n.T("window.5h"),
			UsedPercent: usage.RateLimit.PrimaryWindow.UsedPercent,
			Remaining:   remaining,
		}
		if usage.RateLimit.PrimaryWindow.ResetAt > 0 {
			t := time.Unix(int64(usage.RateLimit.PrimaryWindow.ResetAt), 0).Local()
			w.ResetsAt = &t
		}
		if usage.RateLimit.PrimaryWindow.ResetAfterSeconds > 0 {
			w.ResetSeconds = int64(usage.RateLimit.PrimaryWindow.ResetAfterSeconds)
		}
		result.Windows = append(result.Windows, w)
	}

	if usage.RateLimit.SecondaryWindow.UsedPercent > 0 || usage.RateLimit.SecondaryWindow.ResetAt > 0 {
		remaining := max(0, int(100-usage.RateLimit.SecondaryWindow.UsedPercent+0.5))
		w := UsageWindow{
			Label:       i18n.T("window.weekly"),
			UsedPercent: usage.RateLimit.SecondaryWindow.UsedPercent,
			Remaining:   remaining,
		}
		if usage.RateLimit.SecondaryWindow.ResetAt > 0 {
			t := time.Unix(int64(usage.RateLimit.SecondaryWindow.ResetAt), 0).Local()
			w.ResetsAt = &t
		}
		if usage.RateLimit.SecondaryWindow.ResetAfterSeconds > 0 {
			w.ResetSeconds = int64(usage.RateLimit.SecondaryWindow.ResetAfterSeconds)
		}
		result.Windows = append(result.Windows, w)
	}

	return result
}

func doCodexRequest(creds *auth.CodexCredentials) ([]byte, int, error) {
	req, err := http.NewRequest("GET", codexUsageURL, nil)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CodexBar")
	if creds.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", creds.AccountID)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return body, resp.StatusCode, nil
}

type codexUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow struct {
			UsedPercent       float64 `json:"used_percent"`
			ResetAt           float64 `json:"reset_at"`
			ResetAfterSeconds float64 `json:"reset_after_seconds"`
		} `json:"primary_window"`
		SecondaryWindow struct {
			UsedPercent       float64 `json:"used_percent"`
			ResetAt           float64 `json:"reset_at"`
			ResetAfterSeconds float64 `json:"reset_after_seconds"`
		} `json:"secondary_window"`
	} `json:"rate_limit"`
}
