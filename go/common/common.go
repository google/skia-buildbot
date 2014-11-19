// Common tool initialization.
// import only from package main.
package common

import (
	"flag"
	"net"
	"time"

	"github.com/golang/glog"
	metrics "github.com/rcrowley/go-metrics"
)

const SAMPLE_PERIOD = time.Minute

func Init() {
	flag.Parse()
	defer glog.Flush()
	flag.VisitAll(func(f *flag.Flag) {
		glog.Infof("Flags: --%s=%v", f.Name, f.Value)
	})
}

func InitWithMetrics(appName, graphiteServer string) {
	Init()

	addr, _ := net.ResolveTCPAddr("tcp", graphiteServer)

	// Runtime metrics.
	registry := metrics.NewRegistry()
	metrics.RegisterRuntimeMemStats(registry)
	go metrics.CaptureRuntimeMemStats(registry, SAMPLE_PERIOD)
	go metrics.Graphite(registry, SAMPLE_PERIOD, appName, addr)

	// Uptime.
	uptimeGuage := metrics.GetOrRegisterGaugeFloat64("uptime", registry)
	go func() {
		startTime := time.Now()
		uptimeGuage.Update(0)
		for _ = range time.Tick(SAMPLE_PERIOD) {
			uptimeGuage.Update(time.Now().Sub(startTime).Seconds())
		}
	}()
}
