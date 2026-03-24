package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/emarc09/fuelcheck/internal/auth"
	"github.com/emarc09/fuelcheck/internal/i18n"
)

const (
	geminiLoadCodeAssistURL    = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	geminiRetrieveUserQuotaURL = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
)

var geminiTiers = []struct {
	Name   string
	Models []string
}{
	{"3-Flash", []string{"gemini-3-flash-preview"}},
	{"Flash", []string{"gemini-2.5-flash", "gemini-2.5-flash-lite", "gemini-2.0-flash"}},
	{"Pro", []string{"gemini-2.5-pro", "gemini-3-pro-preview"}},
}

func FetchGeminiUsage() *ProviderResult {
	result := &ProviderResult{Provider: "Gemini", OK: false}

	creds, err := auth.GetGeminiCredentials()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.TokenRefreshed = creds.TokenRefreshed
	result.Email = creds.Email

	if creds.AccessToken == "" && creds.APIKey != "" {
		result.AuthMethod = "API Key"
		result.Hint = i18n.T("err.gemini.api_key_hint")
		result.OK = true
		return result
	}

	result.AuthMethod = "OAuth (Google Account)"

	project, tier, err := geminiLoadCodeAssist(creds.AccessToken)
	if err != nil {
		result.Error = i18n.Tf("err.gemini.load_error", err)
		return result
	}

	result.GeminiTier = tier

	buckets, err := geminiRetrieveUserQuota(creds.AccessToken, project)
	if err != nil {
		result.Error = i18n.Tf("err.gemini.quota_error", err)
		return result
	}

	bucketMap := make(map[string]quotaBucket)
	for _, b := range buckets {
		bucketMap[b.ModelID] = b
	}

	result.RawJSON = map[string]interface{}{
		"tier":            tier,
		"project":         project,
		"buckets":         buckets,
		"token_refreshed": creds.TokenRefreshed,
	}

	for _, tierDef := range geminiTiers {
		for _, modelID := range tierDef.Models {
			bucket, ok := bucketMap[modelID]
			if !ok {
				continue
			}

			usedPct := (1 - bucket.RemainingFraction) * 100
			remainingPct := bucket.RemainingFraction * 100

			model := ModelQuota{
				TierName:         tierDef.Name,
				ModelID:          modelID,
				UsedPercent:      usedPct,
				RemainingPercent: remainingPct,
			}

			if bucket.ResetTime != "" {
				if t := ParseISO8601(bucket.ResetTime); t != nil {
					model.ResetsAt = t
				}
			}

			result.Models = append(result.Models, model)
			break
		}
	}

	result.OK = true
	return result
}

func geminiLoadCodeAssist(token string) (project, tier string, err error) {
	payload := map[string]interface{}{
		"metadata": map[string]string{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest("POST", geminiLoadCodeAssistURL, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("Gemini token invalid or expired (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("loadCodeAssist responded with status %d", resp.StatusCode)
	}

	var result struct {
		CurrentTier struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"currentTier"`
		CloudAICompanionProject string `json:"cloudaicompanionProject"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("error parsing response: %w", err)
	}

	tier = result.CurrentTier.Name
	if tier == "" {
		tier = result.CurrentTier.ID
	}
	project = result.CloudAICompanionProject

	return project, tier, nil
}

func geminiRetrieveUserQuota(token, project string) ([]quotaBucket, error) {
	payload := map[string]string{"project": project}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", geminiRetrieveUserQuotaURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("retrieveUserQuota responded with status %d", resp.StatusCode)
	}

	var result struct {
		Buckets []quotaBucket `json:"buckets"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result.Buckets, nil
}

type quotaBucket struct {
	ModelID           string  `json:"modelId"`
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}
