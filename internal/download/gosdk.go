package download

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GoVersion represents one downloadable Go release.
type GoVersion struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

type goRelease struct {
	Version string   `json:"version"`
	Stable  bool     `json:"stable"`
	Files   []goFile `json:"files"`
}

type goFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

// ListGoVersions fetches available Go SDK versions from go.dev.
func ListGoVersions(ctx context.Context) ([]GoVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://go.dev/dl/?mode=json", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch go versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go.dev returned status %d", resp.StatusCode)
	}

	var releases []goRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode go versions: %w", err)
	}

	versions := make([]GoVersion, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, GoVersion{
			Version: r.Version,
			Stable:  r.Stable,
		})
	}
	return versions, nil
}

// DownloadGoSDK downloads and extracts a Go SDK to targetDir.
func DownloadGoSDK(ctx context.Context, version string, targetDir string, onProgress OnProgress) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	filename := fmt.Sprintf("%s.%s-%s.%s", version, goos, goarch, ext)
	url := fmt.Sprintf("https://go.dev/dl/%s", filename)

	if onProgress != nil {
		onProgress(Progress{
			Tool:    "go",
			Stage:   "downloading",
			Message: fmt.Sprintf("Downloading %s...", filename),
		})
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download go sdk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	totalBytes := resp.ContentLength

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "gosdk-*."+ext)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Download with progress
	var received int64
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			tmpFile.Close()
			return err
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				tmpFile.Close()
				return fmt.Errorf("write temp file: %w", writeErr)
			}
			received += int64(n)
			if onProgress != nil {
				onProgress(Progress{
					Tool:          "go",
					Stage:         "downloading",
					BytesReceived: received,
					BytesTotal:    totalBytes,
					Percent:       calcPercent(received, totalBytes),
					Message:       fmt.Sprintf("Downloading %s...", filename),
				})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tmpFile.Close()
			return fmt.Errorf("read response: %w", readErr)
		}
	}
	tmpFile.Close()

	if onProgress != nil {
		onProgress(Progress{
			Tool:    "go",
			Stage:   "extracting",
			Percent: 100,
			Message: "Extracting Go SDK...",
		})
	}

	// Remove existing go dir if present
	goDir := filepath.Join(targetDir, "go")
	os.RemoveAll(goDir)

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	if ext == "tar.gz" {
		if err := extractTarGz(tmpPath, targetDir); err != nil {
			return fmt.Errorf("extract tar.gz: %w", err)
		}
	} else {
		return fmt.Errorf("zip extraction not yet implemented")
	}

	return nil
}

func extractTarGz(archivePath string, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			// Cap extraction to header.Size to prevent decompression bombs.
			if _, err := io.Copy(out, io.LimitReader(tr, header.Size+1)); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}
