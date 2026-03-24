package providers

import (
	"net/http"
	"time"
)

// httpClient is a shared HTTP client with a 30-second timeout.
// Reused across providers for TCP connection pooling.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}
