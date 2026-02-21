package project

import (
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var skippedDirectories = map[string]struct{}{
	".git":         {},
	".idea":        {},
	".vscode":      {},
	"node_modules": {},
	"vendor":       {},
}

// RunTarget describes one runnable package target.
type RunTarget struct {
	Package string
	Command string
	Path    string
}

// DiscoverRunTargets scans a project tree and returns runnable main packages.
func DiscoverRunTargets(ctx context.Context, root string) ([]RunTarget, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("discover targets context: %w", err)
	}
	if root == "" {
		return nil, fmt.Errorf("root path is required")
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	directories := make([]string, 0)
	if err := filepath.WalkDir(absoluteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}

		name := entry.Name()
		if _, ok := skippedDirectories[name]; ok {
			return filepath.SkipDir
		}
		if strings.HasPrefix(name, ".") && path != absoluteRoot {
			return filepath.SkipDir
		}
		directories = append(directories, path)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk project tree: %w", err)
	}

	targets := make([]RunTarget, 0)
	for _, directory := range directories {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("discover targets context: %w", err)
		}
		runnable, err := isRunnableMainPackage(directory)
		if err != nil {
			return nil, fmt.Errorf("inspect package %s: %w", directory, err)
		}
		if !runnable {
			continue
		}

		relativePath, err := filepath.Rel(absoluteRoot, directory)
		if err != nil {
			return nil, fmt.Errorf("resolve relative path: %w", err)
		}
		relativePath = filepath.ToSlash(relativePath)
		packagePath := "."
		command := "go run ."
		if relativePath != "." {
			packagePath = "./" + relativePath
			command = "go run " + packagePath
		}

		targets = append(targets, RunTarget{
			Package: packagePath,
			Command: command,
			Path:    directory,
		})
	}

	slices.SortFunc(targets, func(a, b RunTarget) int {
		if a.Package < b.Package {
			return -1
		}
		if a.Package > b.Package {
			return 1
		}
		return 0
	})

	return targets, nil
}

func isRunnableMainPackage(directory string) (bool, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, fmt.Errorf("read directory: %w", err)
	}

	goFiles := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if !matchesBuildConstraints(filepath.Join(directory, name)) {
			continue
		}
		goFiles = append(goFiles, filepath.Join(directory, name))
	}
	if len(goFiles) == 0 {
		return false, nil
	}

	fileSet := token.NewFileSet()
	packageName := ""
	hasMainFunc := false

	for _, filePath := range goFiles {
		fileNode, err := parser.ParseFile(fileSet, filePath, nil, 0)
		if err != nil {
			return false, fmt.Errorf("parse go file: %w", err)
		}

		if packageName == "" {
			packageName = fileNode.Name.Name
		}

		for _, declaration := range fileNode.Decls {
			funcDecl, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if funcDecl.Name.Name == "main" && funcDecl.Recv == nil {
				hasMainFunc = true
			}
		}
	}

	return packageName == "main" && hasMainFunc, nil
}

func matchesBuildConstraints(filePath string) bool {
	ctx := build.Default
	match, err := ctx.MatchFile(filepath.Dir(filePath), filepath.Base(filePath))
	if err != nil {
		return true
	}
	return match
}
