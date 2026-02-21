package telemetry

import (
	"fmt"
	"sync"
	"time"
)

// StartupEvent captures startup timing.
type StartupEvent struct {
	StartedAt   time.Time
	CompletedAt time.Time
	Duration    time.Duration
}

// RunEvent captures run trigger and first-output timings.
type RunEvent struct {
	RunID             string
	TriggeredAt       time.Time
	FirstOutputAt     time.Time
	TimeToFirstOutput time.Duration
}

type runState struct {
	triggeredAt   time.Time
	firstOutputAt time.Time
	closed        bool
}

// Recorder tracks startup and run latency events in memory.
type Recorder struct {
	mu   sync.Mutex
	runs map[string]runState
}

// NewRecorder creates a telemetry recorder.
func NewRecorder() *Recorder {
	return &Recorder{
		runs: make(map[string]runState),
	}
}

// MarkStartupComplete computes startup duration from a provided start time.
func (r *Recorder) MarkStartupComplete(startedAt time.Time) StartupEvent {
	completedAt := time.Now()
	if completedAt.Before(startedAt) {
		completedAt = startedAt
	}
	return StartupEvent{
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Duration:    completedAt.Sub(startedAt),
	}
}

// MarkRunTriggered stores the trigger timestamp for a run.
func (r *Recorder) MarkRunTriggered(runID string, triggeredAt time.Time) error {
	if runID == "" {
		return fmt.Errorf("run ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.runs[runID] = runState{
		triggeredAt: triggeredAt,
	}
	return nil
}

// MarkFirstOutput records first output and returns an event.
// The boolean return reports whether this call emitted a new event.
func (r *Recorder) MarkFirstOutput(runID string, firstOutputAt time.Time) (RunEvent, bool, error) {
	if runID == "" {
		return RunEvent{}, false, fmt.Errorf("run ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.runs[runID]
	if !ok {
		return RunEvent{}, false, fmt.Errorf("run not found")
	}
	if state.closed {
		return RunEvent{}, false, nil
	}
	if firstOutputAt.Before(state.triggeredAt) {
		firstOutputAt = state.triggeredAt
	}

	delete(r.runs, runID)

	return RunEvent{
		RunID:             runID,
		TriggeredAt:       state.triggeredAt,
		FirstOutputAt:     firstOutputAt,
		TimeToFirstOutput: firstOutputAt.Sub(state.triggeredAt),
	}, true, nil
}
