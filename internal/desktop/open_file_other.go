//go:build !darwin

package desktop

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func openGoFileDialog(ctx context.Context) (string, error) {
	return runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Open Go File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Go Files (*.go)", Pattern: "*.go"},
		},
	})
}
