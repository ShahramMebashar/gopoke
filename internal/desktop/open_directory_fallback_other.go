//go:build !darwin

package desktop

import (
	"context"
	"time"
)

func preferredOpenDirectoryDialog(ctx context.Context) (string, bool, error) {
	_ = ctx
	return "", false, nil
}

func fallbackOpenDirectoryDialog(initialDialogDuration time.Duration) (string, bool, error) {
	_ = initialDialogDuration
	return "", false, nil
}
