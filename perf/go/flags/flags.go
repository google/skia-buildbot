// flags utilities.
package flags

import (
	"flag"

	"github.com/golang/glog"
)

// Log echoes all the flags and their values into the logs.
func Log() {
	flag.VisitAll(func(f *flag.Flag) {
		glog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})
}
