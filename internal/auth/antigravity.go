package auth

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// AntigravityCredentials holds the locally-discovered Antigravity credentials.
type AntigravityCredentials struct {
	CSRFToken string
	PID       int
	Ports     []int // TCP ports the process is listening on
	ExtPort   int   // extension_server_port from args (if present)
}

// GetAntigravityCredentials detects a running Antigravity process and extracts its credentials.
func GetAntigravityCredentials() (*AntigravityCredentials, error) {
	pid, csrfToken, extPort, err := findAntigravityProcess()
	if err != nil {
		return nil, err
	}

	ports, err := discoverPorts(pid)
	if err != nil || len(ports) == 0 {
		return nil, fmt.Errorf("no se encontraron puertos TCP para Antigravity (PID %d)", pid)
	}

	return &AntigravityCredentials{
		CSRFToken: csrfToken,
		PID:       pid,
		Ports:     ports,
		ExtPort:   extPort,
	}, nil
}

// antigravityBinaryName returns the process name to search for based on OS/arch.
func antigravityBinaryName() string {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "language_server_macos_arm"
		}
		return "language_server_macos"
	case "linux":
		return "language_server_linux"
	default:
		return "language_server"
	}
}

func findAntigravityProcess() (pid int, csrfToken string, extPort int, err error) {
	ctx, cancel := contextWithTimeout(2 * time.Second)
	defer cancel()

	binaryName := antigravityBinaryName()
	out, err := exec.CommandContext(ctx, "pgrep", "-lf", binaryName).Output()
	if err != nil {
		return 0, "", 0, fmt.Errorf("Antigravity no está corriendo")
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if !isAntigravityProcess(line) {
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

		csrf := extractArg(line, "--csrf_token")
		if csrf == "" {
			continue
		}

		ext := 0
		if extStr := extractArg(line, "--extension_server_port"); extStr != "" {
			ext, _ = strconv.Atoi(extStr)
		}

		return p, csrf, ext, nil
	}

	return 0, "", 0, fmt.Errorf("Antigravity no está corriendo")
}

func isAntigravityProcess(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "--app_data_dir antigravity") ||
		strings.Contains(lower, "--app_data_dir .antigravity") {
		return true
	}
	if strings.Contains(lower, "/antigravity/") || strings.Contains(lower, ".antigravity/") {
		return true
	}
	return false
}

func extractArg(line, flag string) string {
	re := regexp.MustCompile(regexp.QuoteMeta(flag) + `\s+(\S+)`)
	match := re.FindStringSubmatch(line)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func discoverPorts(pid int) ([]int, error) {
	ctx, cancel := contextWithTimeout(2 * time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		// On Linux, use ss instead of lsof.
		cmd = exec.CommandContext(ctx, "ss", "-tlnp",
			fmt.Sprintf("( sport = :%d )", pid))
	default:
		cmd = exec.CommandContext(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN",
			"-a", "-p", strconv.Itoa(pid))
	}

	out, err := cmd.Output()
	if err != nil {
		// Fallback: try lsof on Linux too (some distros have it).
		if runtime.GOOS == "linux" {
			cmd2 := exec.CommandContext(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN",
				"-a", "-p", strconv.Itoa(pid))
			out, err = cmd2.Output()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	portRe := regexp.MustCompile(`:(\d+)\s+\(LISTEN\)`)
	// Also match ss output format: *:PORT
	ssPortRe := regexp.MustCompile(`\*:(\d+)\s`)

	seen := make(map[int]bool)
	var ports []int

	for _, line := range strings.Split(string(out), "\n") {
		var matches []string
		if m := portRe.FindStringSubmatch(line); len(m) >= 2 {
			matches = m
		} else if m := ssPortRe.FindStringSubmatch(line); len(m) >= 2 {
			matches = m
		}
		if len(matches) < 2 {
			continue
		}
		port, err := strconv.Atoi(matches[1])
		if err != nil || seen[port] {
			continue
		}
		seen[port] = true
		ports = append(ports, port)
	}

	return ports, nil
}
