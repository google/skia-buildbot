// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/bazel"
	"go.skia.org/infra/task_driver/go/td"
)

func TestBuildPackages_RBE(t *testing.T) {
	workspace := t.TempDir()
	execRoot := t.TempDir()
	binDir := filepath.Join(execRoot, "bazel-out", "k8-fastbuild", "bin", "app")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	macTmp := t.TempDir()
	winTmp := t.TempDir()
	pkgs := []*pkgSpec{
		{
			bazelPlatform: "@rules_go//go/toolchain:darwin_amd64",
			cipdPlatform:  "mac-amd64",
			tmpDir:        macTmp,
		},
		{
			bazelPlatform: "@rules_go//go/toolchain:windows_amd64",
			cipdPlatform:  "windows-amd64",
			tmpDir:        winTmp,
		},
	}

	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			require.Equal(t, "bazelisk", cmd.Name)
			require.Equal(t, workspace, cmd.Dir)
			switch cmd.Args[0] {
			case "info":
				_, err := cmd.Stdout.Write([]byte(execRoot + "\n"))
				return err
			case "build":
				if slices.Contains(cmd.Args, "--platforms=@rules_go//go/toolchain:darwin_amd64") {
					return os.WriteFile(filepath.Join(binDir, "app"), []byte("mac-binary"), 0755)
				}
				if slices.Contains(cmd.Args, "--platforms=@rules_go//go/toolchain:windows_amd64") {
					// Leave the previous platform's "app" file in place to verify
					// only the files reported by cquery are copied to winTmp.
					return os.WriteFile(filepath.Join(binDir, "app.exe"), []byte("win-binary"), 0755)
				}
			case "cquery":
				if cmd.Stderr != nil {
					_, _ = cmd.Stderr.Write([]byte("INFO: Analyzed target //app:app\n"))
				}
				if slices.Contains(cmd.Args, "--platforms=@rules_go//go/toolchain:darwin_amd64") {
					_, err := cmd.Stdout.Write([]byte("bazel-out/k8-fastbuild/bin/app/app\n"))
					return err
				}
				if slices.Contains(cmd.Args, "--platforms=@rules_go//go/toolchain:windows_amd64") {
					_, err := cmd.Stdout.Write([]byte("bazel-out/k8-fastbuild/bin/app/app.exe\n"))
					return err
				}
			}
			return nil
		})
		ctx = exec.NewContext(ctx, mockRun.Run)

		bzl, err := bazel.New(ctx, workspace, "", bazel.BazelOptions{})
		require.NoError(t, err)
		defer bzl.Cleanup()

		err = buildPackages(ctx, bzl, []string{"//app:app"}, pkgs, true)
		require.NoError(t, err)

		cmds := mockRun.Commands()
		require.Len(t, cmds, 5)
		require.Equal(t, []string{"info", "execution_root"}, cmds[0].Args)
		require.Equal(t, []string{
			"build",
			"--config=remote",
			"--sandbox_base=/dev/shm",
			"--google_default_credentials",
			"--platforms=@rules_go//go/toolchain:darwin_amd64",
			"//app:app",
		}, cmds[1].Args)
		require.Equal(t, []string{
			"cquery",
			"--config=remote",
			"--sandbox_base=/dev/shm",
			"--google_default_credentials",
			"--platforms=@rules_go//go/toolchain:darwin_amd64",
			"--output=files",
			"//app:app",
		}, cmds[2].Args)
		require.Equal(t, []string{
			"build",
			"--config=remote",
			"--sandbox_base=/dev/shm",
			"--google_default_credentials",
			"--platforms=@rules_go//go/toolchain:windows_amd64",
			"//app:app",
		}, cmds[3].Args)
		require.Equal(t, []string{
			"cquery",
			"--config=remote",
			"--sandbox_base=/dev/shm",
			"--google_default_credentials",
			"--platforms=@rules_go//go/toolchain:windows_amd64",
			"--output=files",
			"//app:app",
		}, cmds[4].Args)

		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)

	// Verify each platform directory contains only its own outputs.
	macEntries, err := os.ReadDir(macTmp)
	require.NoError(t, err)
	require.Len(t, macEntries, 1)
	require.Equal(t, "app", macEntries[0].Name())
	macContent, err := os.ReadFile(filepath.Join(macTmp, "app"))
	require.NoError(t, err)
	require.Equal(t, "mac-binary", string(macContent))

	winEntries, err := os.ReadDir(winTmp)
	require.NoError(t, err)
	require.Len(t, winEntries, 1)
	require.Equal(t, "app.exe", winEntries[0].Name())
	winContent, err := os.ReadFile(filepath.Join(winTmp, "app.exe"))
	require.NoError(t, err)
	require.Equal(t, "win-binary", string(winContent))
}

func TestBuildPackages_EmptyCqueryOutput(t *testing.T) {
	workspace := t.TempDir()
	execRoot := t.TempDir()
	pkgs := []*pkgSpec{
		{
			bazelPlatform: "@rules_go//go/toolchain:linux_amd64",
			cipdPlatform:  "linux-amd64",
			tmpDir:        t.TempDir(),
		},
	}

	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			if cmd.Args[0] == "info" {
				_, err := cmd.Stdout.Write([]byte(execRoot + "\n"))
				return err
			}
			return nil
		})
		ctx = exec.NewContext(ctx, mockRun.Run)

		bzl, err := bazel.New(ctx, workspace, "", bazel.BazelOptions{})
		require.NoError(t, err)
		defer bzl.Cleanup()

		err = buildPackages(ctx, bzl, []string{"//app:app"}, pkgs, false)
		require.ErrorContains(t, err, "no output files found")

		cmds := mockRun.Commands()
		require.Len(t, cmds, 3)
		require.Equal(t, []string{"build", "--platforms=@rules_go//go/toolchain:linux_amd64", "//app:app"}, cmds[1].Args)
		require.Equal(t, []string{"cquery", "--platforms=@rules_go//go/toolchain:linux_amd64", "--output=files", "//app:app"}, cmds[2].Args)
		return nil
	})
	require.Empty(t, res.Exceptions)
}
