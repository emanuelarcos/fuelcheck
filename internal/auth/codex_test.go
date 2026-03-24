package auth

import "testing"

func TestEmailFromJWT(t *testing.T) {
	// This is a real JWT structure (payload only matters).
	// payload: {"email": "test@example.com", "sub": "12345"}
	// base64url of that: eyJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJzdWIiOiIxMjM0NSJ9
	token := "eyJhbGciOiJSUzI1NiJ9.eyJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJzdWIiOiIxMjM0NSJ9.signature"

	email := emailFromJWT(token)
	if email != "test@example.com" {
		t.Errorf("emailFromJWT() = %q, want %q", email, "test@example.com")
	}
}

func TestEmailFromJWTInvalid(t *testing.T) {
	tests := []string{
		"",
		"not-a-jwt",
		"header.!!!invalid-base64!!!.sig",
		"header..sig",
	}
	for _, tt := range tests {
		email := emailFromJWT(tt)
		if email != "" {
			t.Errorf("emailFromJWT(%q) = %q, want empty", tt, email)
		}
	}
}

func TestExtractClaudeOAuthToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"nested structure",
			`{"claudeAiOauth": {"accessToken": "tok_123"}}`,
			"tok_123",
		},
		{
			"flat structure",
			`{"accessToken": "tok_456"}`,
			"tok_456",
		},
		{
			"raw token string",
			"sk-ant-some-long-token-value-here",
			"sk-ant-some-long-token-value-here",
		},
		{
			"empty json",
			`{}`,
			"",
		},
		{
			"short string",
			"short",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractClaudeOAuthToken([]byte(tt.input))
			if got != tt.want {
				t.Errorf("extractClaudeOAuthToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
