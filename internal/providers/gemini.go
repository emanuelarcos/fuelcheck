package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/emarc09/fuelcheck/internal/auth"
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

// FetchGeminiUsage fetches usage data from the Gemini API.
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
		result.Hint = "API key no soporta la API de cuota. Consultá https://aistudio.google.com"
		result.OK = true
		return result
	}

	result.AuthMethod = "OAuth (Google Account)"

	project, tier, err := geminiLoadCodeAssist(creds.AccessToken)
	if err != nil {
		result.Error = fmt.Sprintf("error al cargar CodeAssist: %v", err)
		return result
	}

	result.GeminiTier = tier

	buckets, err := geminiRetrieveUserQuota(creds.AccessToken, project)
	if err != nil {
		result.Error = fmt.Sprintf("error al obtener cuota: %v", err)
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
		return "", "", fmt.Errorf("error de conexión: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("error al leer respuesta: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("token de Gemini inválido o expirado (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("loadCodeAssist respondió con status %d", resp.StatusCode)
	}

	var result struct {
		CurrentTier struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"currentTier"`
		CloudAICompanionProject string `json:"cloudaicompanionProject"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("error al parsear respuesta: %w", err)
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
		return nil, fmt.Errorf("error de conexión: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer respuesta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("retrieveUserQuota respondió con status %d", resp.StatusCode)
	}

	var result struct {
		Buckets []quotaBucket `json:"buckets"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error al parsear respuesta: %w", err)
	}

	return result.Buckets, nil
}

type quotaBucket struct {
	ModelID           string  `json:"modelId"`
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}
