//go:build !wails

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gopoke/internal/app"
	"gopoke/internal/runner"
)

func main() {
	if runner.RunWorkerModeIfEnabled() {
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application := app.New()
	if err := application.Start(ctx); err != nil {
		slog.Error("application startup failed", "error", err)
		os.Exit(1)
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.DefaultShutdownTimeout)
	defer cancel()
	if err := application.Stop(shutdownCtx); err != nil {
		slog.Error("application shutdown failed", "error", err)
		os.Exit(1)
	}
}
