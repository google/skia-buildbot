// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.skia.org/infra/go/git"
)

// getWorkspaceDir returns the workspace directory when running via bazel run.
func getWorkspaceDir() string {
	return os.Getenv("BUILD_WORKSPACE_DIRECTORY")
}

// fetchPatch fetches the diff of the last commit and current modifications.
func fetchPatch(ctx context.Context, baseCommit string, contextLines int) (string, error) {
	patch, err := runGitDiff(ctx, baseCommit, contextLines)
	if err != nil {
		return "", err
	}

	commitMessage, err := runGitLog(ctx)
	if err != nil {
		return patch, nil
	}

	return fmt.Sprintf("%s\n\n%s", commitMessage, patch), nil
}

// runGitCommand executes a git command with the given arguments and returns its output.
func runGitCommand(ctx context.Context, args ...string) (string, error) {
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return "", fmt.Errorf("git.Executable failed: %w", err)
	}
	cmd := exec.Command(gitExec, args...)
	if dir := getWorkspaceDir(); dir != "" {
		cmd.Dir = dir
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(
			"git %v failed: %w, output: %s",
			args,
			err,
			out.String(),
		)
	}
	return out.String(), nil
}

// runGitLog executes a git log command to get the last commit message.
func runGitLog(ctx context.Context) (string, error) {
	return runGitCommand(ctx, "log", "-1", "--pretty=%B")
}

// runGitDiff executes a git diff command with the given arguments.
func runGitDiff(
	ctx context.Context,
	baseCommit string,
	contextLines int,
) (string, error) {
	return runGitCommand(ctx, "diff", fmt.Sprintf("--unified=%d", contextLines), baseCommit)
}

// untrackedFiles returns a list of untracked files in the git repository.
func untrackedFiles(ctx context.Context) ([]string, error) {
	output, err := runGitCommand(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return nil, nil
	}
	return strings.Split(output, "\n"), nil
}
