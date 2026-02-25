package testutil

import (
	"context"
	"testing"
	"time"
)

type hasDeadline interface {
	Deadline() (deadline time.Time, ok bool)
}

// TestRunContext returns a context bounded by the test deadline (or 10s fallback).
// Leaves 1s margin before the deadline for cleanup.
func TestRunContext(t testing.TB) (context.Context, context.CancelFunc) {
	t.Helper()
	if dl, ok := t.(hasDeadline); ok {
		if deadline, hasIt := dl.Deadline(); hasIt {
			return context.WithDeadline(context.Background(), deadline.Add(-1*time.Second))
		}
	}
	return context.WithTimeout(context.Background(), 10*time.Second)
}
