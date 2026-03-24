
```
  (  ┏━╸╻ ╻┏━╸╻  ┏━╸╻ ╻┏━╸┏━╸╻┏
 )\) ┣╸ ┃ ┃┣╸ ┃  ┃  ┣━┫┣╸ ┃  ┣┻┓
((_) ╹  ┗━┛┗━╸┗━╸┗━╸╹ ╹┗━╸┗━╸╹ ╹
```

**Check your AI subscription usage from the terminal.**

<!--
[![Go Version](https://img.shields.io/github/go-mod/go-version/emanuelarcos/fuelcheck)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/emanuelarcos/fuelcheck)](https://github.com/emanuelarcos/fuelcheck/releases)
-->

---

fuelcheck queries **Claude**, **Codex (ChatGPT)**, **Gemini**, and **Antigravity** in parallel to show how much of your rate limits you've used — right from your terminal.

<!-- SCREENSHOT
To generate a screenshot or GIF of fuelcheck in action:

Option 1: Simple screenshot
  ./fuelcheck | tee /dev/tty | cat    # then take a terminal screenshot

Option 2: Using charmbracelet/vhs (recommended for GIFs)
  1. Install vhs: brew install charmbracelet/tap/vhs
  2. Create a tape file (fuelcheck.tape):
       Output fuelcheck.gif
       Set Width 900
       Set Height 600
       Type "./fuelcheck"
       Enter
       Sleep 3s
  3. Run: vhs fuelcheck.tape
  4. Replace this comment block with:
       ![fuelcheck demo](fuelcheck.gif)
-->

## Features

- **4 providers** — Claude, Codex, Gemini, Antigravity
- **Parallel fetching** — all providers queried concurrently via goroutines
- **Terminal UI** — styled cards, color-coded progress bars ([lipgloss](https://github.com/charmbracelet/lipgloss))
- **JSON output** — `--json` flag for scripting and piping to `jq`
- **Auto credential discovery** — reads tokens from keychains, config files, and environment variables
- **Token refresh** — automatically refreshes expired OAuth tokens (Codex, Gemini)
- **Filter by provider** — query only what you need: `fuelcheck claude`
- **Shell completion** — tab-complete provider names (via [cobra](https://github.com/spf13/cobra))
- **Cross-platform** — macOS and Linux

## Quick Start

```bash
curl -fsSL https://raw.githubusercontent.com/emanuelarcos/fuelcheck/main/install.sh | sh
fuelcheck
```

## Installation

### One-line install (recommended)

Detects your OS and architecture, downloads the latest release, and installs to `/usr/local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/emanuelarcos/fuelcheck/main/install.sh | sh
```

To install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/emanuelarcos/fuelcheck/main/install.sh | sh -s v1.0.0
```

### Pre-built binaries

Download the binary for your platform from [Releases](https://github.com/emanuelarcos/fuelcheck/releases), extract, and move to your PATH:

```bash
tar -xzf fuelcheck_darwin_arm64.tar.gz
sudo mv fuelcheck /usr/local/bin/
```

### Go install

If you have Go installed:

```bash
go install github.com/emanuelarcos/fuelcheck/cmd/fuelcheck@latest
```

### From source

```bash
git clone https://github.com/emanuelarcos/fuelcheck.git
cd fuelcheck
go build -o fuelcheck ./cmd/fuelcheck/
sudo mv fuelcheck /usr/local/bin/
```

<!--
### Homebrew (coming soon)

```bash
brew install emanuelarcos/tap/fuelcheck
```
-->

## Usage

```bash
fuelcheck                    # All providers
fuelcheck claude             # Claude only
fuelcheck claude codex       # Claude and Codex
fuelcheck --json             # All providers, JSON output
fuelcheck gemini --json      # Gemini only, JSON output
fuelcheck --version          # Print version
fuelcheck --help             # Show help
```

### JSON output

The `--json` flag outputs raw API responses, useful for scripting:

```bash
# Get Claude's remaining percentage
fuelcheck claude --json | jq '.Claude.usage.five_hour.utilization'

# Monitor all providers in a script
fuelcheck --json | jq '.[] | {provider: .provider, error: .error}'
```

## Providers

| Provider     | What it shows                        | Auth method                  | Prerequisite                  |
|-------------|--------------------------------------|------------------------------|-------------------------------|
| Claude      | 5-hour and weekly usage windows      | OAuth token (auto-detected)  | Logged into Claude Code       |
| Codex       | 5-hour and weekly usage windows      | OAuth token + auto-refresh   | Logged into Codex CLI         |
| Gemini      | Per-model tier quotas (Flash, Pro)   | OAuth token + auto-refresh   | Logged into Gemini CLI        |
| Antigravity | Per-model quotas (all available models) | Local process detection   | Desktop app running           |

<details>
<summary><strong>Claude</strong> — credential discovery</summary>

Credentials are resolved in this order:

1. `CLAUDE_CODE_OAUTH_TOKEN` env var
2. `CLAUDE_CODE_SESSION_ACCESS_TOKEN` env var
3. `ANTHROPIC_AUTH_TOKEN` env var
4. `CLAUDE_ACCESS_TOKEN` env var
5. `CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR` (file descriptor)
6. macOS Keychain (`Claude Code-credentials`)
7. `~/.claude/.credentials.json`
8. `~/.config/claude/.credentials.json`
9. Web session fallback via `CLAUDE_AI_SESSION_KEY` or `CLAUDE_WEB_SESSION_KEY`

Account metadata (plan, email) is read from `~/.claude.json`.

</details>

<details>
<summary><strong>Codex</strong> — credential discovery</summary>

Reads OAuth tokens from:

- `~/.codex/auth.json`
- `~/.config/codex/auth.json`

If the API returns 401/403, fuelcheck automatically refreshes the token using the refresh_token and persists it back to disk.

Email is extracted from the JWT `id_token` in the auth file.

</details>

<details>
<summary><strong>Gemini</strong> — credential discovery</summary>

Reads OAuth credentials from:

- `~/.gemini/oauth_creds.json`
- `~/.config/gemini/oauth_creds.json`

Also checks `GEMINI_API_KEY` / `GOOGLE_API_KEY` env vars (API keys can't query the quota API, so only a hint is shown).

If the token is expired, fuelcheck refreshes it using the Gemini CLI's public OAuth client credentials. These are discovered by:

1. `GEMINI_OAUTH_CLIENT_ID` / `GEMINI_OAUTH_CLIENT_SECRET` env vars
2. Resolving the `gemini` binary and finding `oauth2.js` in the package
3. `npm root -g` to locate the global install
4. Well-known paths for npm, nvm, yarn, pnpm installs

</details>

<details>
<summary><strong>Antigravity</strong> — credential discovery</summary>

Antigravity (formerly Windsurf/Codeium) runs a local language server. fuelcheck detects it by:

1. Finding the process via `pgrep` (`language_server_macos_arm` on Apple Silicon, `language_server_linux` on Linux)
2. Extracting the `--csrf_token` from the process arguments
3. Discovering TCP ports via `lsof` (macOS) or `ss` (Linux)
4. Querying the local gRPC endpoint with the CSRF token

**The Antigravity desktop app must be running** for this provider to work.

</details>

## Project Structure

```
.
├── cmd/fuelcheck/            # CLI entrypoint (cobra, flags, --json)
│   └── main.go
├── internal/
│   ├── auth/                 # Credential discovery per provider
│   │   ├── claude.go         # Keychain, env vars, config files, web session
│   │   ├── codex.go          # auth.json parsing, token refresh, JWT email
│   │   ├── gemini.go         # OAuth creds, token refresh, CLI client discovery
│   │   └── antigravity.go    # Process detection, CSRF extraction, port discovery
│   ├── providers/            # API clients
│   │   ├── claude.go         # OAuth + web session APIs
│   │   ├── codex.go          # ChatGPT backend API
│   │   ├── gemini.go         # GCP CodeAssist + quota APIs
│   │   ├── antigravity.go    # Local gRPC endpoint
│   │   └── types.go          # Shared types (ProviderResult, UsageWindow, ModelQuota)
│   └── ui/                   # Terminal rendering (lipgloss)
│       └── render.go         # Cards, progress bars, banner, color coding
├── .github/workflows/
│   ├── ci.yml                # Tests + build on push/PR
│   └── release.yml           # GoReleaser on tag push
├── .goreleaser.yml           # Cross-compilation config (darwin/linux, amd64/arm64)
├── install.sh                # One-line installer script
├── go.mod
├── LICENSE
└── README.md
```

## Contributing

Contributions are welcome!

```bash
# Fork and clone
git clone https://github.com/YOUR_USERNAME/fuelcheck.git
cd fuelcheck

# Create a branch
git checkout -b feat/my-feature

# Run tests
go test ./...

# Build and verify
go build -o fuelcheck ./cmd/fuelcheck/
go vet ./...
```

### Adding a new provider

1. Create `internal/auth/yourprovider.go` — credential discovery logic
2. Create `internal/providers/yourprovider.go` — API client that returns a `*ProviderResult`
3. Register it in `cmd/fuelcheck/main.go` (add to `allProviders` map and `providerOrder` slice)
4. Add a metadata case in `internal/ui/render.go` → `renderMetadata()`
5. Add tests in `internal/providers/yourprovider_test.go`

### Opening a PR

- Keep commits focused and descriptive
- Make sure `go test ./...` and `go vet ./...` pass
- Describe what the change does and why

## Disclaimer

fuelcheck uses **internal/undocumented APIs** from each provider to fetch usage data. These endpoints can change or break without notice. This tool is not affiliated with or endorsed by Anthropic, OpenAI, Google.

## License

[MIT](LICENSE)
