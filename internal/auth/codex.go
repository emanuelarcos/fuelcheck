package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emarc09/fuelcheck/internal/i18n"
)

const (
	codexTokenURL = "https://auth.openai.com/oauth/token"
	codexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
)

// CodexCredentials holds the resolved Codex/OpenAI authentication.
type CodexCredentials struct {
	AccessToken string
	AccountID   string
	Email       string
	AuthPath    string // path to auth.json for persistence

	rawAuth map[string]interface{} // full auth file for re-serialization
}

// GetCodexCredentials reads authentication from ~/.codex/auth.json.
func GetCodexCredentials() (*CodexCredentials, error) {
	paths := codexAuthPaths()
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		creds := &CodexCredentials{
			AuthPath: p,
			rawAuth:  raw,
		}

		// Extract tokens.
		if tokens, ok := raw["tokens"].(map[string]interface{}); ok {
			if at, ok := tokens["access_token"].(string); ok {
				creds.AccessToken = at
			}
			if aid, ok := tokens["account_id"].(string); ok {
				creds.AccountID = aid
			}
		}

		// Extract email.
		creds.Email = creds.extractEmail()

		if creds.AccessToken != "" {
			return creds, nil
		}
	}

	return nil, fmt.Errorf("%s", i18n.T("err.codex.no_auth"))
}

// RefreshToken refreshes the access token using the refresh token.
// Returns true if the token was successfully refreshed.
func (c *CodexCredentials) RefreshToken() error {
	tokens, ok := c.rawAuth["tokens"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no se encontró tokens en auth.json")
	}

	refreshToken, ok := tokens["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return fmt.Errorf("no se encontró refresh_token")
	}

	// POST to token endpoint.
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {codexClientID},
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(codexTokenURL, form)
	if err != nil {
		return fmt.Errorf("error al refrescar token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh falló con status %d", resp.StatusCode)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("error al decodificar respuesta de refresh: %w", err)
	}

	// Update in-memory credentials.
	c.AccessToken = result.AccessToken
	tokens["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		tokens["refresh_token"] = result.RefreshToken
	}

	// Persist to disk.
	data, err := json.MarshalIndent(c.rawAuth, "", "  ")
	if err != nil {
		return nil // token refreshed in memory, just can't persist
	}
	_ = os.WriteFile(c.AuthPath, data, 0600)

	return nil
}

// extractEmail extracts the user's email from auth data.
func (c *CodexCredentials) extractEmail() string {
	// Try user.email.
	if user, ok := c.rawAuth["user"].(map[string]interface{}); ok {
		if email, ok := user["email"].(string); ok && email != "" {
			return email
		}
	}

	// Try top-level email.
	if email, ok := c.rawAuth["email"].(string); ok && email != "" {
		return email
	}

	// Try decoding JWT id_token.
	if tokens, ok := c.rawAuth["tokens"].(map[string]interface{}); ok {
		if idToken, ok := tokens["id_token"].(string); ok {
			email := emailFromJWT(idToken)
			if email != "" {
				return email
			}
		}
	}

	return ""
}

// emailFromJWT extracts the email from a JWT token's payload.
func emailFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}

	// Add padding.
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	data, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}

	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(data, &claims); err != nil {
		return ""
	}

	return claims.Email
}

// codexAuthPaths returns possible auth file locations.
func codexAuthPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".codex", "auth.json"),
		filepath.Join(home, ".config", "codex", "auth.json"),
	}
}
