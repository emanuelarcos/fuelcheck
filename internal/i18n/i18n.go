package i18n

import (
	"fmt"
	"os"
	"strings"
)

// Lang represents a supported language.
type Lang string

const (
	EN Lang = "en"
	ES Lang = "es"
)

// current holds the active language. Defaults to EN until Detect() or Set() is called.
var current Lang = EN

// Detect resolves the language from system locale environment variables.
// Priority: FUELCHECK_LANG > LC_ALL > LANG > LANGUAGE > default (en).
func Detect() Lang {
	for _, env := range []string{"FUELCHECK_LANG", "LC_ALL", "LANG", "LANGUAGE"} {
		if v := os.Getenv(env); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "es") {
				current = ES
				return current
			}
			// Any non-empty, non-Spanish locale → English.
			if env == "FUELCHECK_LANG" || len(v) >= 2 {
				current = EN
				return current
			}
		}
	}
	current = EN
	return current
}

// Set forces a specific language, overriding detection.
func Set(lang string) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "es", "español", "spanish":
		current = ES
	default:
		current = EN
	}
}

// Current returns the active language.
func Current() Lang {
	return current
}

// T returns the translated string for the given key.
// Falls back to the English string, then to the key itself.
func T(key string) string {
	if s, ok := translations[current][key]; ok {
		return s
	}
	if s, ok := translations[EN][key]; ok {
		return s
	}
	return key
}

// Tf returns the translated string formatted with the given args.
func Tf(key string, args ...interface{}) string {
	return fmt.Sprintf(T(key), args...)
}

// translations holds all translatable strings.
var translations = map[Lang]map[string]string{
	EN: {
		// UI labels
		"ui.tagline":         "AI subscription usage",
		"ui.remaining":       "%d%% remaining",
		"ui.resets_at":       "Resets: %s",
		"ui.plan":            "Plan: ",
		"ui.email":           "Email: ",
		"ui.tier":            "Tier: ",
		"ui.source":          "Source: ",
		"ui.auth":            "Auth: ",
		"ui.token_refreshed": "(token refreshed)",

		// Time formatting
		"time.am":  "AM",
		"time.pm":  "PM",
		"time.jan": "Jan", "time.feb": "Feb", "time.mar": "Mar",
		"time.apr": "Apr", "time.may": "May", "time.jun": "Jun",
		"time.jul": "Jul", "time.aug": "Aug", "time.sep": "Sep",
		"time.oct": "Oct", "time.nov": "Nov", "time.dec": "Dec",

		// Usage window labels
		"window.5h":            "5-hour usage limit",
		"window.weekly":        "Weekly usage limit",
		"window.weekly_sonnet": "Weekly Sonnet limit",
		"window.weekly_opus":   "Weekly Opus limit",
		"window.usage":         "Usage limit",

		// CLI
		"cli.short":            "Check your AI subscription usage",
		"cli.long_desc":        "Queries your AI subscription usage in parallel\nfor Claude, Codex, Gemini, Antigravity and Windsurf.",
		"cli.providers":        "Available providers: claude, codex, gemini, antigravity, windsurf",
		"cli.json_flag":        "Output in JSON format",
		"cli.lang_flag":        "Language (en, es). Auto-detected from system locale",
		"cli.all_failed":       "all providers failed",
		"cli.unknown_provider": "unknown provider: %q\nAvailable: %s",

		// Provider errors
		"err.connection":        "connection error: %v",
		"err.connection_retry":  "connection error after refresh: %v",
		"err.read_response":     "error reading response: %v",
		"err.parse_json":        "error parsing JSON: %v",
		"err.api_status":        "API responded with status %d",
		"err.too_many_requests": "Too many requests — wait a few minutes and try again",
		"err.create_request":    "error creating request: %v",

		// Claude errors
		"err.claude.no_creds":         "no Claude credentials found.\nSet CLAUDE_CODE_OAUTH_TOKEN or log in with Claude Code",
		"err.claude.oauth_invalid":    "OAuth token invalid and no web session available",
		"err.claude.no_creds_found":   "no Claude credentials found",
		"err.claude.orgs_error":       "error fetching organizations: %v",
		"err.claude.orgs_status":      "error fetching organizations: status %d",
		"err.claude.no_orgs":          "no organizations found",
		"err.claude.usage_error":      "error fetching usage: %v",
		"err.claude.usage_read_error": "error reading usage response: %v",
		"err.claude.usage_status":     "error fetching usage: status %d",
		"err.claude.usage_parse":      "error parsing usage: %v",

		// Codex errors
		"err.codex.no_auth":       "~/.codex/auth.json not found.\nLog in with Codex CLI first",
		"err.codex.token_expired": "token expired and could not be refreshed: %v",

		// Gemini errors
		"err.gemini.no_creds":     "no Gemini credentials found.\nSet GEMINI_API_KEY or log in with Gemini CLI",
		"err.gemini.api_key_hint": "API key does not support the quota API. Check https://aistudio.google.com",
		"err.gemini.load_error":   "error loading CodeAssist: %v",
		"err.gemini.quota_error":  "error fetching quota: %v",

		// Antigravity errors
		"err.antigravity.not_running": "Antigravity is not running",
		"err.antigravity.no_ports":    "no TCP ports found for Antigravity (PID %d)",
		"err.antigravity.no_connect":  "could not connect to Antigravity's local API",
		"err.antigravity.no_models":   "no models with quota found",

		// Windsurf errors
		"err.windsurf.not_running":   "Windsurf is not running",
		"err.windsurf.not_installed": "Windsurf is not installed",
		"err.windsurf.no_ports":      "no TCP ports found for Windsurf (PID %d)",
		"err.windsurf.no_csrf":       "could not extract CSRF token from Windsurf process",
		"err.windsurf.no_api_key":    "could not read Windsurf API key",
		"err.windsurf.no_connect":    "could not connect to Windsurf's local API",
		"err.windsurf.no_data":       "no usage data found for Windsurf",
	},

	ES: {
		// UI labels
		"ui.tagline":         "Uso de suscripciones IA",
		"ui.remaining":       "%d%% restante",
		"ui.resets_at":       "Se restablecerá: %s",
		"ui.plan":            "Plan: ",
		"ui.email":           "Email: ",
		"ui.tier":            "Tier: ",
		"ui.source":          "Fuente: ",
		"ui.auth":            "Auth: ",
		"ui.token_refreshed": "(token refrescado)",

		// Time formatting
		"time.am":  "a.m.",
		"time.pm":  "p.m.",
		"time.jan": "ene", "time.feb": "feb", "time.mar": "mar",
		"time.apr": "abr", "time.may": "may", "time.jun": "jun",
		"time.jul": "jul", "time.aug": "ago", "time.sep": "sep",
		"time.oct": "oct", "time.nov": "nov", "time.dec": "dic",

		// Usage window labels
		"window.5h":            "Límite de uso de 5 horas",
		"window.weekly":        "Límite de uso semanal",
		"window.weekly_sonnet": "Límite semanal Sonnet",
		"window.weekly_opus":   "Límite semanal Opus",
		"window.usage":         "Límite de uso",

		// CLI
		"cli.short":            "Consulta el estado de uso de tus suscripciones de IA",
		"cli.long_desc":        "Consulta en paralelo el estado de uso de tus suscripciones\nde Claude, Codex, Gemini, Antigravity y Windsurf.",
		"cli.providers":        "Proveedores disponibles: claude, codex, gemini, antigravity, windsurf",
		"cli.json_flag":        "Salida en formato JSON",
		"cli.lang_flag":        "Idioma (en, es). Se detecta automáticamente del sistema",
		"cli.all_failed":       "todos los proveedores fallaron",
		"cli.unknown_provider": "proveedor desconocido: %q\nDisponibles: %s",

		// Provider errors
		"err.connection":        "error de conexión: %v",
		"err.connection_retry":  "error de conexión tras refresh: %v",
		"err.read_response":     "error al leer respuesta: %v",
		"err.parse_json":        "error al parsear JSON: %v",
		"err.api_status":        "API respondió con status %d",
		"err.too_many_requests": "Too many requests — esperá unos minutos e intentá de nuevo",
		"err.create_request":    "error al crear request: %v",

		// Claude errors
		"err.claude.no_creds":         "no se encontraron credenciales de Claude.\nConfigurá CLAUDE_CODE_OAUTH_TOKEN o iniciá sesión con Claude Code",
		"err.claude.oauth_invalid":    "OAuth token inválido y no hay sesión web disponible",
		"err.claude.no_creds_found":   "no se encontraron credenciales de Claude",
		"err.claude.orgs_error":       "error al obtener organizaciones: %v",
		"err.claude.orgs_status":      "error al obtener organizaciones: status %d",
		"err.claude.no_orgs":          "no se encontraron organizaciones",
		"err.claude.usage_error":      "error al obtener uso: %v",
		"err.claude.usage_read_error": "error al leer respuesta de uso: %v",
		"err.claude.usage_status":     "error al obtener uso: status %d",
		"err.claude.usage_parse":      "error al parsear uso: %v",

		// Codex errors
		"err.codex.no_auth":       "no se encontró ~/.codex/auth.json.\nIniciá sesión con Codex CLI primero",
		"err.codex.token_expired": "token expirado y no se pudo refrescar: %v",

		// Gemini errors
		"err.gemini.no_creds":     "no se encontraron credenciales de Gemini.\nConfigurá GEMINI_API_KEY o iniciá sesión con Gemini CLI",
		"err.gemini.api_key_hint": "API key no soporta la API de cuota. Consultá https://aistudio.google.com",
		"err.gemini.load_error":   "error al cargar CodeAssist: %v",
		"err.gemini.quota_error":  "error al obtener cuota: %v",

		// Antigravity errors
		"err.antigravity.not_running": "Antigravity no está corriendo",
		"err.antigravity.no_ports":    "no se encontraron puertos TCP para Antigravity (PID %d)",
		"err.antigravity.no_connect":  "no se pudo conectar a la API local de Antigravity",
		"err.antigravity.no_models":   "no se encontraron modelos con cuota",

		// Windsurf errors
		"err.windsurf.not_running":   "Windsurf no está corriendo",
		"err.windsurf.not_installed": "Windsurf no está instalado",
		"err.windsurf.no_ports":      "no se encontraron puertos TCP para Windsurf (PID %d)",
		"err.windsurf.no_csrf":       "no se pudo extraer el token CSRF del proceso de Windsurf",
		"err.windsurf.no_api_key":    "no se pudo leer la clave API de Windsurf",
		"err.windsurf.no_connect":    "no se pudo conectar a la API local de Windsurf",
		"err.windsurf.no_data":       "no se encontraron datos de uso para Windsurf",
	},
}

// MonthName returns the localized month abbreviation (1-indexed: 1=Jan).
func MonthName(month int) string {
	keys := []string{
		"", "time.jan", "time.feb", "time.mar", "time.apr", "time.may", "time.jun",
		"time.jul", "time.aug", "time.sep", "time.oct", "time.nov", "time.dec",
	}
	if month < 1 || month > 12 {
		return ""
	}
	return T(keys[month])
}
