package auth

import (
	"context"
	"time"
)

// contextWithTimeout creates a context with the given timeout.
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
