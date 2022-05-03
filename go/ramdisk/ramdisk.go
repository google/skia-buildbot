package ramdisk

import (
	"context"
	"fmt"
	"os"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// New mounts a ram disk and returns the mount location and a cleanup function
// which should be deferred immediately after checking the returned error.
func New(ctx context.Context) (string, func(), error) {
	location, err := os.MkdirTemp(os.TempDir(), "ramdisk")
	if err != nil {
		return "", nil, skerr.Wrap(err)
	}
	fmt.Println("sudo is required in order to mount the ram disk.")
	cmd := executil.CommandContext(ctx, "sudo", "mount", "-t", "tmpfs", "-o", "size=10m", "tmpfs", location)
	if output, err := cmd.Output(); err != nil {
		if err2 := os.RemoveAll(location); err2 != nil {
			return "", nil, skerr.Wrapf(err, "failed to mount ram disk and failed to remove mount point with: %s", err2)
		}
		return "", nil, skerr.Wrapf(err, "output: %s", string(output))
	}
	return location, func() {
		cmd := executil.CommandContext(ctx, "sudo", "umount", location)
		if output, err := cmd.Output(); err != nil {
			sklog.Errorf("Failed to unmount ram disk: %s; output: %s", err, string(output))
			return
		}
		if err := os.RemoveAll(location); err != nil {
			sklog.Errorf("Failed to delete ram disk mount point: %s", err)
		}
	}, nil
}
