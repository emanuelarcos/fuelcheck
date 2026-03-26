package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/emarc09/fuelcheck/internal/i18n"
)

// WindsurfCredentials holds the locally-discovered Windsurf credentials.
type WindsurfCredentials struct {
	CSRFToken string
	APIKey    string
	PID       int
	Ports     []int // TCP ports the process is listening on
}

// GetWindsurfCredentials detects a running Windsurf process and extracts its credentials.
func GetWindsurfCredentials() (*WindsurfCredentials, error) {
	pid, extPort, err := findWindsurfProcess()
	if err != nil {
		return nil, err
	}

	csrfToken, err := extractWindsurfCSRF(pid)
	if err != nil {
		return nil, err
	}

	apiKey, err := readWindsurfAPIKey()
	if err != nil {
		return nil, err
	}

	ports, err := discoverPorts(pid)
	if err != nil || len(ports) == 0 {
		// Fall back to the extension server port if lsof finds nothing.
		if extPort > 0 {
			ports = []int{extPort}
		} else {
			return nil, fmt.Errorf("%s", i18n.Tf("err.windsurf.no_ports", pid))
		}
	}

	return &WindsurfCredentials{
		CSRFToken: csrfToken,
		APIKey:    apiKey,
		PID:       pid,
		Ports:     ports,
	}, nil
}

func findWindsurfProcess() (pid int, extPort int, err error) {
	ctx, cancel := contextWithTimeout(2 * time.Second)
	defer cancel()

	binaryName := antigravityBinaryName()
	out, err := exec.CommandContext(ctx, "pgrep", "-lf", binaryName).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("%s", i18n.T("err.windsurf.not_running"))
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if !isWindsurfProcess(line) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		p, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		ext := 0
		if extStr := extractArg(line, "--extension_server_port"); extStr != "" {
			ext, _ = strconv.Atoi(extStr)
		}

		return p, ext, nil
	}

	return 0, 0, fmt.Errorf("%s", i18n.T("err.windsurf.not_running"))
}

func isWindsurfProcess(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "--ide_name windsurf") {
		return true
	}
	if strings.Contains(lower, "/windsurf.app/") || strings.Contains(lower, "/windsurf/bin/") {
		return true
	}
	return false
}

// extractWindsurfCSRF reads the WINDSURF_CSRF_TOKEN from the process environment.
func extractWindsurfCSRF(pid int) (string, error) {
	ctx, cancel := contextWithTimeout(2 * time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-E").Output()
	if err != nil {
		return "", fmt.Errorf("%s", i18n.T("err.windsurf.no_csrf"))
	}

	const prefix = "WINDSURF_CSRF_TOKEN="
	for _, field := range strings.Fields(string(out)) {
		if strings.HasPrefix(field, prefix) {
			token := strings.TrimPrefix(field, prefix)
			if token != "" {
				return token, nil
			}
		}
	}

	return "", fmt.Errorf("%s", i18n.T("err.windsurf.no_csrf"))
}

// readWindsurfAPIKey retrieves the API key from Windsurf's local state database.
func readWindsurfAPIKey() (string, error) {
	dbPath, err := windsurfDBPath()
	if err != nil {
		return "", err
	}

	ctx, cancel := contextWithTimeout(3 * time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sqlite3", "-noheader", dbPath,
		"SELECT value FROM ItemTable WHERE key = 'windsurfAuthStatus'").Output()
	if err != nil {
		return "", fmt.Errorf("%s", i18n.T("err.windsurf.no_api_key"))
	}

	var status struct {
		APIKey string `json:"apiKey"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(string(out))), &status) != nil || status.APIKey == "" {
		return "", fmt.Errorf("%s", i18n.T("err.windsurf.no_api_key"))
	}

	return status.APIKey, nil
}

func windsurfDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%s", i18n.T("err.windsurf.not_installed"))
	}

	var dbPath string
	switch runtime.GOOS {
	case "darwin":
		dbPath = filepath.Join(home, "Library", "Application Support", "Windsurf", "User", "globalStorage", "state.vscdb")
	case "linux":
		dbPath = filepath.Join(home, ".config", "Windsurf", "User", "globalStorage", "state.vscdb")
	default:
		return "", fmt.Errorf("%s", i18n.T("err.windsurf.not_installed"))
	}

	if _, err := os.Stat(dbPath); err != nil {
		return "", fmt.Errorf("%s", i18n.T("err.windsurf.not_installed"))
	}

	return dbPath, nil
}
