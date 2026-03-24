package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	geminiTokenURL = "https://oauth2.googleapis.com/token"
)

// GeminiCredentials holds the resolved Gemini authentication.
type GeminiCredentials struct {
	APIKey         string
	AccessToken    string
	RefreshToken   string
	ExpiryDate     int64  // Unix timestamp in milliseconds
	OAuthPath      string // path to oauth_creds.json for persistence
	TokenRefreshed bool
	Email          string
}

// GetGeminiCredentials discovers Gemini credentials from env vars and local files.
func GetGeminiCredentials() (*GeminiCredentials, error) {
	creds := &GeminiCredentials{}

	if v := os.Getenv("GEMINI_API_KEY"); v != "" {
		creds.APIKey = v
	}
	if v := os.Getenv("GOOGLE_API_KEY"); v != "" {
		creds.APIKey = v
	}

	oauthPaths := geminiOAuthPaths()
	for _, p := range oauthPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		var oauthFile struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiryDate   int64  `json:"expiry_date"`
			IDToken      string `json:"id_token"`
		}
		if err := json.Unmarshal(data, &oauthFile); err != nil {
			continue
		}

		if oauthFile.AccessToken != "" {
			creds.AccessToken = oauthFile.AccessToken
			creds.RefreshToken = oauthFile.RefreshToken
			creds.ExpiryDate = oauthFile.ExpiryDate
			creds.OAuthPath = p
			if oauthFile.IDToken != "" {
				creds.Email = emailFromJWT(oauthFile.IDToken)
			}
			break
		}
	}

	// Refresh if expired.
	if creds.AccessToken != "" && creds.ExpiryDate > 0 {
		expiryTime := time.Unix(creds.ExpiryDate/1000, 0)
		if time.Now().After(expiryTime) {
			_ = creds.refreshToken()
		}
	}

	if creds.AccessToken == "" && creds.APIKey == "" {
		return nil, fmt.Errorf("no se encontraron credenciales de Gemini.\n" +
			"Configurá GEMINI_API_KEY o iniciá sesión con Gemini CLI")
	}

	return creds, nil
}

func (g *GeminiCredentials) refreshToken() error {
	if g.RefreshToken == "" {
		return fmt.Errorf("no refresh_token disponible")
	}

	clientID, clientSecret, err := getGeminiOAuthCreds()
	if err != nil {
		return fmt.Errorf("no se encontraron credenciales OAuth del CLI de Gemini: %w", err)
	}

	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {g.RefreshToken},
		"grant_type":    {"refresh_token"},
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(geminiTokenURL, form)
	if err != nil {
		return fmt.Errorf("error al refrescar token de Gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh de Gemini falló con status %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("error al decodificar respuesta de refresh: %w", err)
	}

	g.AccessToken = result.AccessToken
	g.ExpiryDate = (time.Now().Unix() + result.ExpiresIn) * 1000
	g.TokenRefreshed = true

	g.persistToken()

	return nil
}

func (g *GeminiCredentials) persistToken() {
	if g.OAuthPath == "" {
		return
	}

	data, err := os.ReadFile(g.OAuthPath)
	if err != nil {
		return
	}

	var existing map[string]interface{}
	if err := json.Unmarshal(data, &existing); err != nil {
		return
	}

	existing["access_token"] = g.AccessToken
	existing["expiry_date"] = g.ExpiryDate

	newData, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return
	}

	tmpPath := g.OAuthPath + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0600); err != nil {
		return
	}
	_ = os.Rename(tmpPath, g.OAuthPath)
}

func getGeminiOAuthCreds() (clientID, clientSecret string, err error) {
	clientID = os.Getenv("GEMINI_OAUTH_CLIENT_ID")
	clientSecret = os.Getenv("GEMINI_OAUTH_CLIENT_SECRET")
	if clientID != "" && clientSecret != "" {
		return clientID, clientSecret, nil
	}

	if id, secret := findOAuthFromGeminiBinary(); id != "" {
		return id, secret, nil
	}

	if id, secret := findOAuthFromNpmGlobal(); id != "" {
		return id, secret, nil
	}

	if id, secret := findOAuthFromGlob(); id != "" {
		return id, secret, nil
	}

	return "", "", fmt.Errorf("no se encontraron las credenciales OAuth del CLI de Gemini")
}

func findOAuthFromGeminiBinary() (string, string) {
	geminiPath, err := exec.LookPath("gemini")
	if err != nil {
		return "", ""
	}

	resolved, err := filepath.EvalSymlinks(geminiPath)
	if err != nil {
		resolved = geminiPath
	}

	dir := filepath.Dir(resolved)
	for i := 0; i < 10; i++ {
		for _, relPath := range oauthJSRelPaths() {
			candidate := filepath.Join(dir, relPath)
			if id, secret := extractOAuthFromFile(candidate); id != "" {
				return id, secret
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", ""
}

func findOAuthFromNpmGlobal() (string, string) {
	ctx, cancel := contextWithTimeout(5 * time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "npm", "root", "-g").Output()
	if err != nil {
		return "", ""
	}

	npmRoot := strings.TrimSpace(string(out))
	for _, relPath := range oauthJSRelPaths() {
		candidate := filepath.Join(npmRoot, relPath)
		if id, secret := extractOAuthFromFile(candidate); id != "" {
			return id, secret
		}
	}

	return "", ""
}

func findOAuthFromGlob() (string, string) {
	home, _ := os.UserHomeDir()

	patterns := []string{
		filepath.Join(home, ".npm/_npx/*/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js"),
		filepath.Join(home, ".npm/_npx/*/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js"),
		filepath.Join(home, ".nvm/versions/node/*/lib/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js"),
		filepath.Join(home, ".nvm/versions/node/*/lib/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js"),
		"/usr/local/lib/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
		"/usr/local/lib/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
		"/opt/homebrew/lib/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
		filepath.Join(home, ".config/yarn/global/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js"),
		filepath.Join(home, ".local/share/pnpm/global/*/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js"),
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if id, secret := extractOAuthFromFile(match); id != "" {
				return id, secret
			}
		}
	}

	return "", ""
}

func extractOAuthFromFile(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}

	content := string(data)

	idRe := regexp.MustCompile(`CLIENT_ID\s*=\s*["']([^"']+)["']`)
	secretRe := regexp.MustCompile(`CLIENT_SECRET\s*=\s*["']([^"']+)["']`)

	idMatch := idRe.FindStringSubmatch(content)
	secretMatch := secretRe.FindStringSubmatch(content)

	if len(idMatch) < 2 || len(secretMatch) < 2 {
		return "", ""
	}

	return idMatch[1], secretMatch[1]
}

func oauthJSRelPaths() []string {
	return []string{
		"node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
		"node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
	}
}

func geminiOAuthPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".gemini", "oauth_creds.json"),
		filepath.Join(home, ".config", "gemini", "oauth_creds.json"),
	}
}
