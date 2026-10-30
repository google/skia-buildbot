// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package copyright_headers provides utilities for checking and adding BSD
// copyright headers to Go source files.
package copyright_headers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
)

var (
	copyrightRe = regexp.MustCompile(`(?m)^// Copyright (?:\(c\) )?\d{4}\b`)
	generatedRe = regexp.MustCompile(`(?m)^//.*DO NOT EDIT`)
	packageRe   = regexp.MustCompile(`(?m)^package\s+\w+`)
	topDeclRe   = regexp.MustCompile(`(?m)^(?:import|const|var|type|func)\b`)
)

// formatHeader returns the standard BSD copyright header for the given year.
func formatHeader(year int) string {
	return fmt.Sprintf(`// Copyright %d Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
`, year)
}

// isMissingCopyrightHeader returns true if the Go file content is not
// generated code and does not contain a copyright notice before the package
// declaration.
func isMissingCopyrightHeader(content string) bool {
	// Check lines before the first top-level declaration (import/const/var/type/func)
	// for a generated-code marker ("// ... DO NOT EDIT"). This covers both standard
	// "// Code generated ... DO NOT EDIT." headers before `package` and SQL exporter
	// "// DO NOT EDIT" comments immediately after `package`.
	headerRegion := content
	if loc := topDeclRe.FindStringIndex(content); loc != nil {
		headerRegion = content[:loc[0]]
	}
	if generatedRe.MatchString(headerRegion) {
		return false
	}

	// Check lines before `package <name>` for an existing copyright notice.
	prePackage := content
	if loc := packageRe.FindStringIndex(content); loc != nil {
		prePackage = content[:loc[0]]
	}
	return !copyrightRe.MatchString(prePackage)
}

// creationYear returns the earliest commit year for relPath in git history,
// falling back to defaultYear if the file has no commit history yet.
func creationYear(ctx context.Context, repoDir, relPath string, defaultYear int) (int, error) {
	output, err := git.CheckoutDir(repoDir).Git(ctx, "log", "--follow", "--format=%ad", "--date=format:%Y", "--", relPath)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		year, err := strconv.Atoi(line)
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		return year, nil
	}
	return defaultYear, nil
}

// findGoFiles returns sorted repo-relative paths to .go files to inspect.
// If paths is non-empty, only those files or directories are searched;
// otherwise all .go files in repoDir are returned.
func findGoFiles(ctx context.Context, repoDir string, paths []string) ([]string, error) {
	args := []string{"ls-files"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	output, err := git.CheckoutDir(repoDir).Git(ctx, args...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".go") {
			files = append(files, line)
		}
	}
	sort.Strings(files)
	return files, nil
}

// AddCopyrightHeadersToGoFiles checks the given repo-relative paths (or all Go
// files in repoDir if relPaths is empty) and prepends a copyright header to any
// non-generated Go file missing one. If dryRun is true, files are not modified
// on disk. It returns the sorted slice of repo-relative paths that were (or
// would be) updated.
func AddCopyrightHeadersToGoFiles(ctx context.Context, repoDir string, relPaths []string, dryRun bool) ([]string, error) {
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return addCopyrightHeadersToGoFiles(ctx, vfs.Local(absRepoDir), absRepoDir, relPaths, dryRun)
}

func addCopyrightHeadersToGoFiles(ctx context.Context, fs vfs.FS, repoDir string, relPaths []string, dryRun bool) ([]string, error) {
	goFiles, err := findGoFiles(ctx, repoDir, relPaths)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defaultYear := time.Now().Year()
	numWorkers := runtime.NumCPU()
	if numWorkers > len(goFiles) {
		numWorkers = len(goFiles)
	}
	if numWorkers == 0 {
		return nil, nil
	}

	pathCh := make(chan string, len(goFiles))
	for _, relPath := range goFiles {
		pathCh <- relPath
	}
	close(pathCh)

	var (
		mu       sync.Mutex
		updated  []string
		firstErr error
		wg       sync.WaitGroup
	)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for relPath := range pathCh {
				modified, err := processFile(ctx, fs, repoDir, relPath, defaultYear, dryRun)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				if modified {
					updated = append(updated, relPath)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if firstErr != nil {
		return nil, skerr.Wrap(firstErr)
	}
	sort.Strings(updated)
	return updated, nil
}

func processFile(ctx context.Context, fs vfs.FS, repoDir, relPath string, defaultYear int, dryRun bool) (bool, error) {
	b, err := vfs.ReadFile(ctx, fs, relPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, skerr.Wrap(err)
	}
	content := string(b)
	if !isMissingCopyrightHeader(content) {
		return false, nil
	}

	year, err := creationYear(ctx, repoDir, relPath, defaultYear)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	header := formatHeader(year)
	newContent := header + "\n" + strings.TrimLeft(content, "\n")

	if !dryRun {
		if err := vfs.WriteFile(ctx, fs, relPath, []byte(newContent)); err != nil {
			return false, skerr.Wrap(err)
		}
	}
	return true, nil
}
