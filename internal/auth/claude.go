package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/emarc09/fuelcheck/internal/i18n"
)

// ClaudeCredentials holds the resolved Claude authentication.
type ClaudeCredentials struct {
	Token  string
	Source string // "oauth", "local oauth", "web"

	// Account metadata (from local files).
	SubscriptionType string
	RateLimitTier    string
	Email            string
}

// ClaudeWebSession holds web session fallback credentials.
type ClaudeWebSession struct {
	SessionKey string
}

// GetClaudeCredentials discovers Claude credentials in priority order.
func GetClaudeCredentials() (*ClaudeCredentials, *ClaudeWebSession, error) {
	creds := &ClaudeCredentials{}

	// 1-4. Environment variables (priority order).
	envVars := []string{
		"CLAUDE_CODE_OAUTH_TOKEN",
		"CLAUDE_CODE_SESSION_ACCESS_TOKEN",
		"ANTHROPIC_AUTH_TOKEN",
		"CLAUDE_ACCESS_TOKEN",
	}
	for _, env := range envVars {
		if v := os.Getenv(env); v != "" {
			creds.Token = v
			creds.Source = "oauth"
			break
		}
	}

	// 5. File descriptor.
	if creds.Token == "" {
		if fdStr := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR"); fdStr != "" {
			var fd int
			if _, err := fmt.Sscanf(fdStr, "%d", &fd); err == nil {
				f := os.NewFile(uintptr(fd), "claude-token-fd")
				if f != nil {
					data, err := io.ReadAll(f)
					f.Close()
					if err == nil && len(data) > 0 {
						creds.Token = strings.TrimSpace(string(data))
						creds.Source = "oauth"
					}
				}
			}
		}
	}

	// 6-8. Local credentials (keychain + files).
	if creds.Token == "" {
		token, source := loadLocalClaudeOAuthToken()
		if token != "" {
			creds.Token = token
			creds.Source = source
		}
	}

	// Load account metadata regardless of token source.
	creds.loadLocalAccount()

	if creds.Token != "" {
		return creds, nil, nil
	}

	// 9. Web session fallback.
	ws := resolveWebSessionKey()
	if ws != nil {
		return nil, ws, nil
	}

	return nil, nil, fmt.Errorf("%s", i18n.T("err.claude.no_creds"))
}

func loadLocalClaudeOAuthToken() (token, source string) {
	if runtime.GOOS == "darwin" {
		token = readKeychainToken()
		if token != "" {
			return token, "local oauth"
		}
	}

	for _, p := range claudeCredentialPaths() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		t := extractClaudeOAuthToken(data)
		if t != "" {
			return t, "local oauth"
		}
	}

	return "", ""
}

func readKeychainToken() string {
	ctx, cancel := contextWithTimeout(5 * time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "security", "find-generic-password",
		"-s", "Claude Code-credentials", "-w")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	return extractClaudeOAuthToken(out)
}

func extractClaudeOAuthToken(data []byte) string {
	s := strings.TrimSpace(string(data))

	var nested struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal([]byte(s), &nested); err == nil && nested.ClaudeAiOauth.AccessToken != "" {
		return nested.ClaudeAiOauth.AccessToken
	}

	var flat struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal([]byte(s), &flat); err == nil && flat.AccessToken != "" {
		return flat.AccessToken
	}

	if !strings.HasPrefix(s, "{") && len(s) > 20 {
		return s
	}

	return ""
}

func claudeCredentialPaths() []string {
	var paths []string

	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".claude")
	}
	paths = append(paths, filepath.Join(configDir, ".credentials.json"))

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		home, _ := os.UserHomeDir()
		xdgConfig = filepath.Join(home, ".config")
	}
	paths = append(paths, filepath.Join(xdgConfig, "claude", ".credentials.json"))

	return paths
}

func (c *ClaudeCredentials) loadLocalAccount() {
	for _, p := range claudeCredentialPaths() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cred struct {
			ClaudeAiOauth struct {
				SubscriptionType string `json:"subscriptionType"`
				RateLimitTier    string `json:"rateLimitTier"`
			} `json:"claudeAiOauth"`
		}
		if err := json.Unmarshal(data, &cred); err == nil {
			if cred.ClaudeAiOauth.SubscriptionType != "" {
				c.SubscriptionType = cred.ClaudeAiOauth.SubscriptionType
			}
			if cred.ClaudeAiOauth.RateLimitTier != "" {
				c.RateLimitTier = cred.ClaudeAiOauth.RateLimitTier
			}
		}
	}

	home, _ := os.UserHomeDir()
	appState, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		return
	}
	var state struct {
		OauthAccount struct {
			SubscriptionType string `json:"subscriptionType"`
			BillingType      string `json:"billingType"`
			RateLimitTier    string `json:"rateLimitTier"`
			EmailAddress     string `json:"emailAddress"`
		} `json:"oauthAccount"`
	}
	if err := json.Unmarshal(appState, &state); err == nil {
		if c.SubscriptionType == "" {
			c.SubscriptionType = state.OauthAccount.SubscriptionType
			if c.SubscriptionType == "" {
				c.SubscriptionType = state.OauthAccount.BillingType
			}
		}
		if c.RateLimitTier == "" {
			c.RateLimitTier = state.OauthAccount.RateLimitTier
		}
		if c.Email == "" {
			c.Email = state.OauthAccount.EmailAddress
		}
	}
}

func resolveWebSessionKey() *ClaudeWebSession {
	for _, env := range []string{"CLAUDE_AI_SESSION_KEY", "CLAUDE_WEB_SESSION_KEY"} {
		if v := os.Getenv(env); strings.HasPrefix(v, "sk-ant-") {
			return &ClaudeWebSession{SessionKey: v}
		}
	}

	if cookie := os.Getenv("CLAUDE_WEB_COOKIE"); cookie != "" {
		for _, part := range strings.Split(cookie, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "sessionKey=sk-ant-") {
				return &ClaudeWebSession{SessionKey: strings.TrimPrefix(part, "sessionKey=")}
			}
		}
	}

	return nil
}
