package providers

import "time"

// UsageWindow represents a single usage window (e.g., 5-hour, weekly).
type UsageWindow struct {
	Label        string     `json:"label"`
	UsedPercent  float64    `json:"used_percent"`
	Remaining    int        `json:"remaining_percent"`
	ResetsAt     *time.Time `json:"resets_at,omitempty"`
	ResetSeconds int64      `json:"reset_seconds,omitempty"`
}

// ModelQuota represents usage for a single model or model tier.
type ModelQuota struct {
	TierName         string     `json:"tier_name"`
	ModelID          string     `json:"model_id"`
	UsedPercent      float64    `json:"used_percent"`
	RemainingPercent float64    `json:"remaining_percent"`
	ResetsAt         *time.Time `json:"resets_at,omitempty"`
}

// ProviderResult holds the usage data from a single provider.
type ProviderResult struct {
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`

	// Common metadata.
	Plan     string `json:"plan,omitempty"`
	PlanType string `json:"plan_type,omitempty"`
	Email    string `json:"email,omitempty"`

	// Claude-specific.
	Tier   string `json:"tier,omitempty"`
	Source string `json:"source,omitempty"`

	// Gemini-specific.
	GeminiTier     string `json:"gemini_tier,omitempty"`
	TokenRefreshed bool   `json:"token_refreshed,omitempty"`
	AuthMethod     string `json:"auth_method,omitempty"`
	Hint           string `json:"hint,omitempty"`

	// Usage windows (Claude & Codex).
	Windows []UsageWindow `json:"windows,omitempty"`

	// Model quotas (Gemini & Antigravity).
	Models []ModelQuota `json:"models,omitempty"`

	// Raw JSON response for --json mode.
	RawJSON interface{} `json:"raw,omitempty"`
}
