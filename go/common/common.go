// Common tool initialization.
// import only from package main.
package common

import (
	"flag"

	"github.com/golang/glog"
)

func Init() {
	flag.Parse()
	defer glog.Flush()
	flag.VisitAll(func(f *flag.Flag) {
		glog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})
}
