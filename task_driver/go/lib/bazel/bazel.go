package bazel

import (
	"context"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

// Bazel provides a Task Driver API for working with Bazel.
type Bazel struct {
	cacheDir          string
	local             bool
	rbeCredentialFile string
	workspace         string
}

// NewWithRamdisk returns a new Bazel instance which uses a ramdisk as the Bazel cache.
//
// Using a ramdisk as the Bazel cache prevents CockroachDB "disk stall detected" errors on GCE VMs
// due to slow I/O.
func NewWithRamdisk(ctx context.Context, workspace string, rbeCredentialFile string) (*Bazel, func(), error) {
	// Create and mount ramdisk.
	//
	// At the time of writing, a full build of the Buildbot repository on an empty Bazel cache takes
	// ~20GB on cache space. Infra-PerCommit-Test-Bazel-Local runs on GCE VMs with 64GB of RAM.
	ramdiskDir, err := os_steps.TempDir(ctx, "", "ramdisk-*")
	if err != nil {
		return nil, nil, err
	}
	if _, err := exec.RunCwd(ctx, workspace, "sudo", "mount", "-t", "tmpfs", "-o", "size=32g", "tmpfs", ramdiskDir); err != nil {
		return nil, nil, err
	}

	// Create Bazel cache directory inside the ramdisk.
	//
	// Using the ramdisk's mount point directly as the Bazel cache causes Bazel to fail with a file
	// permission error. Using a directory within the ramdisk as the Bazel cache prevents this error.
	cacheDir := filepath.Join(ramdiskDir, "bazel-cache")
	if err := os_steps.MkdirAll(ctx, cacheDir); err != nil {
		return nil, nil, err
	}

	absCredentialFile, err := os_steps.Abs(ctx, rbeCredentialFile)
	if err != nil {
		return nil, nil, err
	}
	bzl := &Bazel{
		cacheDir:          cacheDir,
		rbeCredentialFile: absCredentialFile,
		workspace:         workspace,
	}

	cleanup := func() {
		// Shut down the Bazel server. This ensures that there are no processes with open files under
		// the ramdisk, which would otherwise cause a "target is busy" when we unmount the ramdisk.
		if _, err := bzl.Do(ctx, "shutdown"); err != nil {
			td.Fatal(ctx, err)
		}

		if _, err := exec.RunCwd(ctx, workspace, "sudo", "umount", ramdiskDir); err != nil {
			td.Fatal(ctx, err)
		}
		if err := os_steps.RemoveAll(ctx, ramdiskDir); err != nil {
			td.Fatal(ctx, err)
		}
	}
	return bzl, cleanup, nil
}

// New returns a new Bazel instance.
func New(ctx context.Context, workspace string, local bool, rbeCredentialFile string) (*Bazel, func(), error) {
	// cacheDir is a temporary directory for the Bazel cache.
	//
	// We cannot use the default Bazel cache location ($HOME/.cache/bazel)
	// because:
	//
	//  - The cache can be large (>10G).
	//  - Swarming bots have limited storage space on the root partition (15G).
	//  - Because the above, the Bazel build fails with a "no space left on
	//    device" error.
	//  - The Bazel cache under $HOME/.cache/bazel lingers after the tryjob
	//    completes, causing the Swarming bot to be quarantined due to low disk
	//    space.
	//  - Generally, it's considered poor hygiene to leave a bot in a different
	//    state.
	//
	// The temporary directory created by the below function call lives under
	// /mnt/pd0, which has significantly more storage space, and will be wiped
	// after the tryjob completes.
	//
	// Reference: https://docs.bazel.build/versions/master/output_directories.html#current-layout.
	cacheDir, err := os_steps.TempDir(ctx, "", "bazel-user-cache-*")
	if err != nil {
		return nil, nil, err
	}
	absCredentialFile := ""
	if rbeCredentialFile != "" {
		absCredentialFile, err = os_steps.Abs(ctx, rbeCredentialFile)
		if err != nil {
			return nil, nil, err
		}
	}

	cleanup := func() {
		// Clean up the temporary Bazel cache directory when running locally,
		// because during development, we do not want to leave behind a ~10GB Bazel
		// cache directory under /tmp after each run.
		//
		// This is not necessary under Swarming because the temporary directory will
		// be cleaned up automatically.
		if local {
			if err := os_steps.RemoveAll(ctx, cacheDir); err != nil {
				td.Fatal(ctx, err)
			}
		}
	}
	return &Bazel{
		cacheDir:          cacheDir,
		rbeCredentialFile: absCredentialFile,
		workspace:         workspace,
	}, cleanup, nil
}

// Do executes a Bazel subcommand.
func (b *Bazel) Do(ctx context.Context, subCmd string, args ...string) (string, error) {
	cmd := []string{"bazel", "--output_user_root=" + b.cacheDir, subCmd}
	cmd = append(cmd, args...)
	return exec.RunCwd(ctx, b.workspace, cmd...)
}

// DoOnRBE executes a Bazel subcommand on RBE.
func (b *Bazel) DoOnRBE(ctx context.Context, subCmd string, args ...string) (string, error) {
	cmd := []string{"--config=remote"}
	if b.rbeCredentialFile != "" {
		cmd = append(cmd, "--google_credentials="+b.rbeCredentialFile)
	} else {
		cmd = append(cmd, "--google_default_credentials")
	}
	cmd = append(cmd, args...)
	return b.Do(ctx, subCmd, cmd...)
}
