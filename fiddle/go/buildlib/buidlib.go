package buildlib

import (
	"fmt"
	"path/filepath"

	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
)

// BuildLib, given a directory that Skia is checked out into, builds libskia.a
// and fiddle_main.o.
func BuildLib(checkout, depotTools string) error {
	sklog.Info("Starting GNGen")
	if err := buildskia.GNGen(checkout, depotTools, "Release", []string{"is_debug=false"}); err != nil {
		return fmt.Errorf("Failed GN gen: %s", err)
	}

	sklog.Info("Building fiddle")
	if msg, err := buildskia.GNNinjaBuild(checkout, depotTools, "Release", "fiddle", true); err != nil {
		return fmt.Errorf("Failed ninja build of fiddle: %q %s", msg, err)
	}

	sklog.Info("Running the default fiddle")
	runFiddleCmd := &exec.Command{
		Name:      filepath.Join(checkout, "skia", "out", "Release", "fiddle"),
		Dir:       filepath.Join(checkout, "skia"),
		LogStderr: true,
		LogStdout: true,
	}

	if err := exec.Run(runFiddleCmd); err != nil {
		return fmt.Errorf("Failed to run the default fiddle: %s", err)
	}

	return nil
}
