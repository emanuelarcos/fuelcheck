package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/emarc09/fuelcheck/internal/auth"
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
		result.Error = fmt.Sprintf("error de conexión: %v", err)
		return result
	}

	// Token refresh on 401/403.
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		if refreshErr := creds.RefreshToken(); refreshErr != nil {
			result.Error = fmt.Sprintf("token expirado y no se pudo refrescar: %v", refreshErr)
			return result
		}

		body, statusCode, err = doCodexRequest(creds)
		if err != nil {
			result.Error = fmt.Sprintf("error de conexión tras refresh: %v", err)
			return result
		}
	}

	if statusCode != http.StatusOK {
		result.Error = fmt.Sprintf("API respondió con status %d", statusCode)
		return result
	}

	var usage codexUsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		result.Error = fmt.Sprintf("error al parsear JSON: %v", err)
		return result
	}

	var rawJSON interface{}
	_ = json.Unmarshal(body, &rawJSON)
	result.RawJSON = rawJSON

	result.PlanType = capitalize(usage.PlanType)
	result.OK = true

	// Primary window (5 hours).
	if usage.RateLimit.PrimaryWindow.UsedPercent > 0 || usage.RateLimit.PrimaryWindow.ResetAt > 0 {
		remaining := max(0, int(100-usage.RateLimit.PrimaryWindow.UsedPercent+0.5))
		w := UsageWindow{
			Label:       "Límite de uso de 5 horas",
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

	// Secondary window (weekly).
	if usage.RateLimit.SecondaryWindow.UsedPercent > 0 || usage.RateLimit.SecondaryWindow.ResetAt > 0 {
		remaining := max(0, int(100-usage.RateLimit.SecondaryWindow.UsedPercent+0.5))
		w := UsageWindow{
			Label:       "Límite de uso semanal",
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

// doCodexRequest performs the HTTP request to the Codex usage API.
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
