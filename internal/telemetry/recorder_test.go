package telemetry

import (
	"testing"
	"time"
)

func TestMarkStartupComplete(t *testing.T) {
	t.Parallel()

	recorder := NewRecorder()
	startedAt := time.Now().Add(-250 * time.Millisecond)

	event := recorder.MarkStartupComplete(startedAt)

	if event.Duration < 0 {
		t.Fatalf("duration = %s, want non-negative", event.Duration)
	}
	if event.CompletedAt.Before(event.StartedAt) {
		t.Fatalf("completedAt %s before startedAt %s", event.CompletedAt, event.StartedAt)
	}
}

func TestMarkFirstOutput(t *testing.T) {
	t.Parallel()

	recorder := NewRecorder()
	triggeredAt := time.Now()
	if err := recorder.MarkRunTriggered("run-1", triggeredAt); err != nil {
		t.Fatalf("MarkRunTriggered() error = %v", err)
	}

	t.Run("returns event on first output", func(t *testing.T) {
		firstOutputAt := triggeredAt.Add(50 * time.Millisecond)
		event, emitted, err := recorder.MarkFirstOutput("run-1", firstOutputAt)
		if err != nil {
			t.Fatalf("MarkFirstOutput() error = %v", err)
		}
		if !emitted {
			t.Fatal("emitted = false, want true")
		}
		if got, want := event.RunID, "run-1"; got != want {
			t.Fatalf("runID = %q, want %q", got, want)
		}
		if event.TimeToFirstOutput <= 0 {
			t.Fatalf("ttfo = %s, want positive", event.TimeToFirstOutput)
		}
	})

	t.Run("ignores duplicate first-output event", func(t *testing.T) {
		event, emitted, err := recorder.MarkFirstOutput("run-1", triggeredAt.Add(100*time.Millisecond))
		if err != nil {
			t.Fatalf("MarkFirstOutput() error = %v", err)
		}
		if emitted {
			t.Fatal("emitted = true, want false")
		}
		if event.RunID != "" {
			t.Fatalf("event = %+v, want empty", event)
		}
	})
}

func TestMarkFirstOutputForUnknownRun(t *testing.T) {
	t.Parallel()

	recorder := NewRecorder()
	if _, _, err := recorder.MarkFirstOutput("missing", time.Now()); err == nil {
		t.Fatal("MarkFirstOutput() error = nil, want non-nil")
	}
}
