package runner

import (
	"os"
	"os/signal"
	"syscall"
)

const (
	workerModeEnv    = "GOPAD_WORKER_MODE"
	workerProjectEnv = "GOPAD_WORKER_PROJECT"
)

// IsWorkerMode reports whether this process should act as a worker host.
func IsWorkerMode() bool {
	return os.Getenv(workerModeEnv) == "1"
}

// WorkerProjectPath returns the project path passed to worker mode.
func WorkerProjectPath() string {
	return os.Getenv(workerProjectEnv)
}

// RunWorkerModeIfEnabled blocks in worker mode until termination signal arrives.
// It returns true if worker mode was active and handled.
func RunWorkerModeIfEnabled() bool {
	if !IsWorkerMode() {
		return false
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	return true
}
