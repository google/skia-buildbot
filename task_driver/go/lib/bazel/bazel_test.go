// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bazel

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/td"
)

func TestBazel_Do(t *testing.T) {
	workspace := t.TempDir()
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		var runErr error
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			if cmd.Stdout != nil {
				_, err := cmd.Stdout.Write([]byte("stdout-output\n"))
				require.NoError(t, err)
			}
			if cmd.Stderr != nil {
				_, err := cmd.Stderr.Write([]byte("INFO: stderr output\n"))
				require.NoError(t, err)
			}
			return runErr
		})
		ctx = exec.NewContext(ctx, mockRun.Run)

		bzl, err := New(ctx, workspace, "", BazelOptions{})
		require.NoError(t, err)
		defer bzl.Cleanup()

		out, err := bzl.Do(ctx, "info", "execution_root")
		require.NoError(t, err)
		require.Equal(t, "stdout-output\n", out)

		runErr = errors.New("exit status 1")
		out, err = bzl.Do(ctx, "test", "//foo:bar")
		require.ErrorContains(t, err, "exit status 1")
		require.ErrorContains(t, err, "INFO: stderr output")
		require.Equal(t, "stdout-output\n", out)

		cmds := mockRun.Commands()
		require.Len(t, cmds, 2)
		require.Equal(t, workspace, cmds[0].Dir)
		require.Equal(t, "bazelisk", cmds[0].Name)
		require.Equal(t, []string{"info", "execution_root"}, cmds[0].Args)
		require.Equal(t, workspace, cmds[1].Dir)
		require.Equal(t, "bazelisk", cmds[1].Name)
		require.Equal(t, []string{"test", "//foo:bar"}, cmds[1].Args)
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}

func TestBazel_DoOnRBE(t *testing.T) {
	workspace := t.TempDir()
	keyPath := filepath.Join(workspace, "key.json")
	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			if cmd.Stdout != nil {
				_, err := cmd.Stdout.Write([]byte("rbe-stdout\n"))
				require.NoError(t, err)
			}
			if cmd.Stderr != nil {
				_, err := cmd.Stderr.Write([]byte("INFO: stderr output\n"))
				require.NoError(t, err)
			}
			return nil
		})
		ctx = exec.NewContext(ctx, mockRun.Run)

		bzlDefault, err := New(ctx, workspace, "", BazelOptions{})
		require.NoError(t, err)
		defer bzlDefault.Cleanup()

		out, err := bzlDefault.DoOnRBE(ctx, "build", "--platforms=@rules_go//go/toolchain:linux_amd64", "//foo:bar")
		require.NoError(t, err)
		require.Equal(t, "rbe-stdout\n", out)

		bzlCustom, err := New(ctx, workspace, keyPath, BazelOptions{
			RBEConfig: "linux_rbe",
		})
		require.NoError(t, err)
		defer bzlCustom.Cleanup()

		out, err = bzlCustom.DoOnRBE(ctx, "cquery", "--platforms=@rules_go//go/toolchain:linux_amd64", "--output=files", "//foo:bar")
		require.NoError(t, err)
		require.Equal(t, "rbe-stdout\n", out)

		cmds := mockRun.Commands()
		require.Len(t, cmds, 2)
		require.Equal(t, []string{
			"build",
			"--config=remote",
			"--sandbox_base=/dev/shm",
			"--google_default_credentials",
			"--platforms=@rules_go//go/toolchain:linux_amd64",
			"//foo:bar",
		}, cmds[0].Args)
		require.Equal(t, []string{
			"cquery",
			"--config=linux_rbe",
			"--sandbox_base=/dev/shm",
			"--google_credentials=" + keyPath,
			"--platforms=@rules_go//go/toolchain:linux_amd64",
			"--output=files",
			"//foo:bar",
		}, cmds[1].Args)
		return nil
	})
	require.Empty(t, res.Errors)
	require.Empty(t, res.Exceptions)
}
