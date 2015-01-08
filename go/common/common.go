// Common tool initialization.
// import only from package main.
package common

import (
	"flag"
	"net"
	"runtime"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
)

const SAMPLE_PERIOD = time.Minute

// Runs commonly-used initialization metrics.
func Init() {
	flag.Parse()
	defer glog.Flush()
	flag.VisitAll(func(f *flag.Flag) {
		glog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})

	// Use all cores.
	runtime.GOMAXPROCS(runtime.NumCPU())
}

// Runs normal Init functions as well as tracking runtime metrics.
// Sets up Graphite push for go-metrics' DefaultRegistry. Users of
// both InitWithMetrics and metrics.DefaultRegistry will not need to
// run metrics.Graphite(metrics.DefaultRegistry, ...) separately.
func InitWithMetrics(appName string, graphiteServer *string) {
	Init()

	addr, _ := net.ResolveTCPAddr("tcp", *graphiteServer)

	// Runtime metrics.
	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, SAMPLE_PERIOD)
	go metrics.Graphite(metrics.DefaultRegistry, SAMPLE_PERIOD, appName, addr)

	// Uptime.
	uptimeGuage := metrics.GetOrRegisterGaugeFloat64("uptime", metrics.DefaultRegistry)
	go func() {
		startTime := time.Now()
		uptimeGuage.Update(0)
		for _ = range time.Tick(SAMPLE_PERIOD) {
			uptimeGuage.Update(time.Since(startTime).Seconds())
		}
	}()
}
