//go:build darwin

package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func openGoFileDialog(ctx context.Context) (string, error) {
	_ = ctx
	command := exec.Command(
		"osascript",
		"-e",
		`try
  POSIX path of (choose file with prompt "Open Go File" of type {"go"})
on error number -128
  return ""
end try`,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("apple script file dialog: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
