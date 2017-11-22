package buildsecwrap

import (
	"context"
	"fmt"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
)

// Build fiddle_secwrap from source in the given 'dir'. The built exe will be placed in the same dir.
func Build(ctx context.Context, dir string) error {
	sklog.Info("Compiling fiddle_secwrap.")
	runFiddleCmd := &exec.Command{
		Name:       "c++",
		Args:       []string{filepath.Join(dir, "bin", "fiddle_secwrap.cpp"), "-o", filepath.Join(dir, "bin", "fiddle_secwrap")},
		InheritEnv: true,
		LogStderr:  true,
		LogStdout:  true,
	}

	if err := exec.Run(ctx, runFiddleCmd); err != nil {
		return fmt.Errorf("Failed to compile fiddle_secwrap: %s", err)
	}

	return nil
}
