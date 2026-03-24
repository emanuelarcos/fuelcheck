package providers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/emarc09/fuelcheck/internal/auth"
	"github.com/emarc09/fuelcheck/internal/i18n"
)

const (
	antigravityGetUserStatus       = "/exa.language_server_pb.LanguageServerService/GetUserStatus"
	antigravityGetCommandModelCfgs = "/exa.language_server_pb.LanguageServerService/GetCommandModelConfigs"
)

var antigravityRequestBody = []byte(`{"metadata":{"ideName":"antigravity","extensionName":"antigravity","ideVersion":"unknown","locale":"en"}}`)

var antigravityHTTPSClient = &http.Client{
	Timeout: 8 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

var antigravityHTTPClient = &http.Client{Timeout: 8 * time.Second}

func FetchAntigravityUsage() *ProviderResult {
	result := &ProviderResult{Provider: "Antigravity", OK: false}

	creds, err := auth.GetAntigravityCredentials()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	if resp := tryAntigravityEndpoint(creds, antigravityGetUserStatus); resp != nil {
		var usr antigravityUserStatusResponse
		if json.Unmarshal(resp, &usr) == nil && len(usr.UserStatus.CascadeModelConfigData.ClientModelConfigs) > 0 {
			return parseAntigravityUserStatus(result, &usr)
		}
	}

	if resp := tryAntigravityEndpoint(creds, antigravityGetCommandModelCfgs); resp != nil {
		var cfg antigravityModelConfigsResponse
		if json.Unmarshal(resp, &cfg) == nil && len(cfg.ClientModelConfigs) > 0 {
			return parseAntigravityModelConfigs(result, &cfg)
		}
	}

	result.Error = i18n.T("err.antigravity.no_connect")
	return result
}

func tryAntigravityEndpoint(creds *auth.AntigravityCredentials, path string) []byte {
	for _, port := range creds.Ports {
		if resp, err := antigravityPost(antigravityHTTPSClient, "https", port, path, creds.CSRFToken); err == nil {
			return resp
		}
	}
	if creds.ExtPort > 0 {
		if resp, err := antigravityPost(antigravityHTTPClient, "http", creds.ExtPort, path, creds.CSRFToken); err == nil {
			return resp
		}
	}
	return nil
}

func antigravityPost(client *http.Client, scheme string, port int, path, csrfToken string) ([]byte, error) {
	url := fmt.Sprintf("%s://127.0.0.1:%d%s", scheme, port, path)

	req, err := http.NewRequest("POST", url, bytes.NewReader(antigravityRequestBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Codeium-Csrf-Token", csrfToken)
	req.Header.Set("Connect-Protocol-Version", "1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func parseAntigravityUserStatus(result *ProviderResult, usr *antigravityUserStatusResponse) *ProviderResult {
	result.Email = usr.UserStatus.Email

	if usr.UserStatus.PlanStatus.PlanInfo.PlanName != "" {
		result.PlanType = capitalize(usr.UserStatus.PlanStatus.PlanInfo.PlanName)
	}

	result.Models = parseAntigravityModels(usr.UserStatus.CascadeModelConfigData.ClientModelConfigs)
	result.RawJSON = usr

	if len(result.Models) == 0 {
		result.Error = i18n.T("err.antigravity.no_models")
		return result
	}

	result.OK = true
	return result
}

func parseAntigravityModelConfigs(result *ProviderResult, cfg *antigravityModelConfigsResponse) *ProviderResult {
	result.Models = parseAntigravityModels(cfg.ClientModelConfigs)
	result.RawJSON = cfg

	if len(result.Models) == 0 {
		result.Error = i18n.T("err.antigravity.no_models")
		return result
	}

	result.OK = true
	return result
}

func parseAntigravityModels(configs []antigravityModelConfig) []ModelQuota {
	var models []ModelQuota

	for _, cfg := range configs {
		if cfg.QuotaInfo == nil {
			continue
		}

		remainingPct := cfg.QuotaInfo.RemainingFraction * 100
		usedPct := 100 - remainingPct

		model := ModelQuota{
			TierName:         cfg.Label,
			ModelID:          cfg.ModelOrAlias.Model,
			UsedPercent:      usedPct,
			RemainingPercent: remainingPct,
		}

		if cfg.QuotaInfo.ResetTime != "" {
			if t := ParseISO8601(cfg.QuotaInfo.ResetTime); t != nil {
				model.ResetsAt = t
			}
		}

		models = append(models, model)
	}

	return models
}

// --- Response types ---

type antigravityUserStatusResponse struct {
	UserStatus struct {
		Email                  string `json:"email"`
		CascadeModelConfigData struct {
			ClientModelConfigs []antigravityModelConfig `json:"clientModelConfigs"`
		} `json:"cascadeModelConfigData"`
		PlanStatus struct {
			PlanInfo struct {
				PlanName string `json:"planName"`
			} `json:"planInfo"`
		} `json:"planStatus"`
	} `json:"userStatus"`
}

type antigravityModelConfigsResponse struct {
	ClientModelConfigs []antigravityModelConfig `json:"clientModelConfigs"`
}

type antigravityModelConfig struct {
	Label        string `json:"label"`
	ModelOrAlias struct {
		Model string `json:"model"`
	} `json:"modelOrAlias"`
	QuotaInfo *struct {
		RemainingFraction float64 `json:"remainingFraction"`
		ResetTime         string  `json:"resetTime"`
	} `json:"quotaInfo"`
}
