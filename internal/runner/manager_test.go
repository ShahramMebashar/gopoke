package runner

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestHelperWorkerProcess(t *testing.T) {
	if os.Getenv("GOPAD_TEST_HELPER_WORKER") != "1" {
		return
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	os.Exit(0)
}

func TestManagerStartReuseAndStopWorker(t *testing.T) {
	projectPath := t.TempDir()

	manager := NewManager(
		WithCommandFactory(testCommandFactory),
		WithStopTimeout(500*time.Millisecond),
	)

	workerOne, err := manager.StartWorker(context.Background(), projectPath)
	if err != nil {
		t.Fatalf("StartWorker(first) error = %v", err)
	}
	if !workerOne.Running {
		t.Fatal("workerOne.Running = false, want true")
	}
	if workerOne.PID <= 0 {
		t.Fatalf("workerOne.PID = %d, want > 0", workerOne.PID)
	}
	if !manager.IsRunning(projectPath) {
		t.Fatal("IsRunning(projectPath) = false, want true")
	}

	workerTwo, err := manager.StartWorker(context.Background(), projectPath)
	if err != nil {
		t.Fatalf("StartWorker(second) error = %v", err)
	}
	if got, want := workerTwo.PID, workerOne.PID; got != want {
		t.Fatalf("worker reuse PID = %d, want %d", got, want)
	}

	if err := manager.StopWorker(context.Background(), projectPath); err != nil {
		t.Fatalf("StopWorker() error = %v", err)
	}
	if manager.IsRunning(projectPath) {
		t.Fatal("IsRunning(projectPath) = true, want false")
	}
}

func TestManagerStopAll(t *testing.T) {
	projectOne := t.TempDir()
	projectTwo := t.TempDir()

	manager := NewManager(
		WithCommandFactory(testCommandFactory),
		WithStopTimeout(500*time.Millisecond),
	)

	if _, err := manager.StartWorker(context.Background(), projectOne); err != nil {
		t.Fatalf("StartWorker(projectOne) error = %v", err)
	}
	if _, err := manager.StartWorker(context.Background(), projectTwo); err != nil {
		t.Fatalf("StartWorker(projectTwo) error = %v", err)
	}

	if err := manager.StopAll(context.Background()); err != nil {
		t.Fatalf("StopAll() error = %v", err)
	}
	if manager.IsRunning(projectOne) {
		t.Fatal("projectOne still running after StopAll")
	}
	if manager.IsRunning(projectTwo) {
		t.Fatal("projectTwo still running after StopAll")
	}
}

func testCommandFactory(projectPath string) (*exec.Cmd, error) {
	_ = projectPath
	command := exec.Command(os.Args[0], "-test.run=TestHelperWorkerProcess", "--")
	command.Env = append(os.Environ(), "GOPAD_TEST_HELPER_WORKER=1")
	return command, nil
}
