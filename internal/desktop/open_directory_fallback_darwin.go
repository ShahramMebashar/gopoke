//go:build darwin

package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const fallbackDialogThreshold = 250 * time.Millisecond

func preferredOpenDirectoryDialog(ctx context.Context) (string, bool, error) {
	_ = ctx
	path, err := openDirectoryViaAppleScript()
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func fallbackOpenDirectoryDialog(initialDialogDuration time.Duration) (string, bool, error) {
	if initialDialogDuration > fallbackDialogThreshold {
		return "", false, nil
	}

	path, err := openDirectoryViaAppleScript()
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(path) == "" {
		return "", false, nil
	}
	return path, true, nil
}

func openDirectoryViaAppleScript() (string, error) {
	command := exec.Command(
		"osascript",
		"-e",
		`try
  POSIX path of (choose folder with prompt "Open Go Project")
on error number -128
  return ""
end try`,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("apple script directory dialog: %w", err)
	}

	path := strings.TrimSpace(string(output))
	return path, nil
}
