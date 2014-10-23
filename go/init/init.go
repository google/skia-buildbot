// Common tool initialization.
// import only from package main as _.
package init

import (
	"flag"

	"github.com/golang/glog"
)

func init() {
	flag.Parse()
	defer glog.Flush()
	flag.VisitAll(func(f *flag.Flag) {
		glog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})
}
