package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/emarc09/fuelcheck/internal/auth"
	"github.com/emarc09/fuelcheck/internal/i18n"
)

var windsurfHTTPClient = &http.Client{Timeout: 8 * time.Second}

func FetchWindsurfUsage() *ProviderResult {
	result := &ProviderResult{Provider: "Windsurf", OK: false}

	creds, err := auth.GetWindsurfCredentials()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	body := []byte(fmt.Sprintf(
		`{"metadata":{"ideName":"windsurf","extensionName":"windsurf","ideVersion":"1.0.0","extensionVersion":"1.0.0","locale":"en","apiKey":"%s"}}`,
		creds.APIKey,
	))

	resp := tryWindsurfEndpoint(creds, body, antigravityGetUserStatus)
	if resp == nil {
		result.Error = i18n.T("err.windsurf.no_connect")
		return result
	}

	var usr windsurfUserStatusResponse
	if json.Unmarshal(resp, &usr) != nil {
		result.Error = i18n.T("err.windsurf.no_connect")
		return result
	}

	return parseWindsurfUserStatus(result, &usr)
}

func tryWindsurfEndpoint(creds *auth.WindsurfCredentials, body []byte, path string) []byte {
	for _, port := range creds.Ports {
		if resp, err := windsurfPost(port, path, creds.CSRFToken, body); err == nil {
			return resp
		}
	}
	return nil
}

func windsurfPost(port int, path, csrfToken string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Codeium-Csrf-Token", csrfToken)
	req.Header.Set("Connect-Protocol-Version", "1")

	resp, err := windsurfHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func parseWindsurfUserStatus(result *ProviderResult, usr *windsurfUserStatusResponse) *ProviderResult {
	result.Email = usr.UserStatus.Email

	if usr.UserStatus.PlanStatus.PlanInfo.PlanName != "" {
		result.PlanType = capitalize(usr.UserStatus.PlanStatus.PlanInfo.PlanName)
	}

	var models []ModelQuota

	// Daily quota
	if usr.UserStatus.PlanStatus.DailyQuotaResetAtUnix != "" {
		dailyUsed := 100 - usr.UserStatus.PlanStatus.DailyQuotaRemainingPercent
		m := ModelQuota{
			TierName:         "Daily Quota",
			ModelID:          fmt.Sprintf("%.0f%% remaining", usr.UserStatus.PlanStatus.DailyQuotaRemainingPercent),
			UsedPercent:      dailyUsed,
			RemainingPercent: usr.UserStatus.PlanStatus.DailyQuotaRemainingPercent,
		}
		if t := parseUnixTimestamp(usr.UserStatus.PlanStatus.DailyQuotaResetAtUnix); t != nil {
			m.ResetsAt = t
		}
		models = append(models, m)
	}

	// Weekly quota
	if usr.UserStatus.PlanStatus.WeeklyQuotaResetAtUnix != "" {
		weeklyUsed := 100 - usr.UserStatus.PlanStatus.WeeklyQuotaRemainingPercent
		m := ModelQuota{
			TierName:         "Weekly Quota",
			ModelID:          fmt.Sprintf("%.0f%% remaining", usr.UserStatus.PlanStatus.WeeklyQuotaRemainingPercent),
			UsedPercent:      weeklyUsed,
			RemainingPercent: usr.UserStatus.PlanStatus.WeeklyQuotaRemainingPercent,
		}
		if t := parseUnixTimestamp(usr.UserStatus.PlanStatus.WeeklyQuotaResetAtUnix); t != nil {
			m.ResetsAt = t
		}
		models = append(models, m)
	}

	// Flex credits (show available out of monthly allocation)
	monthlyPrompt := usr.PlanInfo.MonthlyPromptCredits
	if monthlyPrompt > 0 {
		available := float64(usr.UserStatus.PlanStatus.AvailableFlexCredits)
		total := float64(monthlyPrompt)
		usedPct := (1 - available/total) * 100
		if usedPct < 0 {
			usedPct = 0
		}
		models = append(models, ModelQuota{
			TierName:         "Prompt Credits",
			ModelID:          fmt.Sprintf("%.0f / %d", available, monthlyPrompt),
			UsedPercent:      usedPct,
			RemainingPercent: 100 - usedPct,
		})
	}

	// Per-model quotas from cascade config (if any have quotaInfo)
	models = append(models, parseAntigravityModels(usr.UserStatus.CascadeModelConfigData.ClientModelConfigs)...)

	if len(models) == 0 {
		result.Error = i18n.T("err.windsurf.no_data")
		return result
	}

	result.Models = models
	result.RawJSON = usr
	result.OK = true
	return result
}

func parseUnixTimestamp(s string) *time.Time {
	unix, err := strconv.ParseInt(s, 10, 64)
	if err != nil || unix == 0 {
		return nil
	}
	t := time.Unix(unix, 0)
	return &t
}

// --- Response types ---

type windsurfUserStatusResponse struct {
	UserStatus struct {
		Email                  string `json:"email"`
		CascadeModelConfigData struct {
			ClientModelConfigs []antigravityModelConfig `json:"clientModelConfigs"`
		} `json:"cascadeModelConfigData"`
		PlanStatus struct {
			PlanInfo struct {
				PlanName string `json:"planName"`
			} `json:"planInfo"`
			AvailableFlexCredits         float64 `json:"availableFlexCredits"`
			DailyQuotaRemainingPercent   float64 `json:"dailyQuotaRemainingPercent"`
			WeeklyQuotaRemainingPercent  float64 `json:"weeklyQuotaRemainingPercent"`
			DailyQuotaResetAtUnix        string  `json:"dailyQuotaResetAtUnix"`
			WeeklyQuotaResetAtUnix       string  `json:"weeklyQuotaResetAtUnix"`
		} `json:"planStatus"`
	} `json:"userStatus"`
	PlanInfo struct {
		MonthlyPromptCredits int `json:"monthlyPromptCredits"`
		MonthlyFlowCredits   int `json:"monthlyFlowCredits"`
	} `json:"planInfo"`
}
