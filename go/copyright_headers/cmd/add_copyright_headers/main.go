// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Command add_copyright_headers adds missing copyright headers to Go files
// in the repository using their creation year in git history.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/go/copyright_headers"
)

func main() {
	var (
		repoDir = flag.String("repo_dir", os.Getenv("BUILD_WORKSPACE_DIRECTORY"), "Root directory of the repository. Defaults to BUILD_WORKSPACE_DIRECTORY or the current working directory.")
		dryRun  = flag.Bool("dry_run", false, "Print files that would be updated without modifying them.")
	)
	flag.Parse()

	if *repoDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
			os.Exit(1)
		}
		*repoDir = wd
	}

	ctx := context.Background()
	updated, err := copyright_headers.AddCopyrightHeadersToGoFiles(ctx, *repoDir, flag.Args(), *dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add copyright headers: %v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		for _, relPath := range updated {
			fmt.Printf("Would update %s\n", relPath)
		}
		fmt.Printf("Would add copyright headers to %d Go files.\n", len(updated))
	} else {
		fmt.Printf("Added copyright headers to %d Go files.\n", len(updated))
	}
}
