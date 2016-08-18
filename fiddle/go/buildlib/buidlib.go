package buildlib

import (
	"fmt"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildskia"
)

// BuildLib, given a directory that Skia is checked out into, builds libskia.a
// and fiddle_main.o.
func BuildLib(checkout, depotTools string) error {
	glog.Info("Starting GNGen")
	if err := buildskia.GNGen(checkout, depotTools, "Release", []string{"is_debug=false", "extra_cflags=\"-g0\""}); err != nil {
		return fmt.Errorf("Failed GN gen: %s", err)
	}

	glog.Info("Building fiddle")
	if msg, err := buildskia.GNNinjaBuild(checkout, depotTools, "Release", "fiddle", true); err != nil {
		return fmt.Errorf("Failed ninja build of fiddle: %q %s", msg, err)
	}
	return nil
}
