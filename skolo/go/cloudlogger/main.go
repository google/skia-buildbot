package main

// CloudLogger takes the log files on the raspberry pi, parses them, and uploads them to Google Cloud Logging.

import (
	"flag"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/logagents"
	"go.skia.org/infra/skolo/go/logparser"
)

var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	persistenceDir = flag.String("persistence_dir", "/var/cloudlogger", "The directory in which persistence data regarding the logging progress should be kept.")
	pollPeriod     = flag.Duration("poll_period", 1*time.Minute, `The period used to poll the log files`)
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	rolloverLogs   = common.NewMultiStringFlag("rollover_logs", nil, "A set of log file paths that may roll over.  e.g. supply run_isolated.log to monitor run_isolated.log and run_isolated.log.1")
)

func main() {

	common.InitWithMust(
		"cloudlogger",
		common.PrometheusOpt(promPort),
		common.CloudLoggingDefaultAuthOpt(local),
	)

	if err := logagents.SetPersistenceDir(*persistenceDir); err != nil {
		sklog.Fatalf("Could not set Persistence Dir: %s", err)
	}

	sklog.Info("Begin Cloud Logging")
	scanners := []logagents.LogScanner{}
	for _, r := range *rolloverLogs {
		scanners = append(scanners, logagents.NewRollover(logparser.ParsePythonLog, cleanupName(r), r, r+".1"))
	}

	scan(scanners, *pollPeriod)
}

// scan executes Scan on all LogScanners on a repeating time clock of period.  This executes indefinitely.
func scan(scanners []logagents.LogScanner, period time.Duration) {
	for range time.Tick(period) {
		sklog.Infof("Waking up to scan logs.")
		for _, s := range scanners {
			sklog.Infof("Scanning %s", s.ReportName())
			if err := s.Scan(sklog.CloudLoggingInstance()); err != nil {
				sklog.Errorf("Problem with log file %s : %s", s.ReportName(), err)
			}
		}
	}
}

// cleanupName removes .log from the end of the given name if it is there.
func cleanupName(s string) string {
	s = strings.Replace(s, ".log", "", -1)
	xs := strings.Split(s, "/")
	return xs[len(xs)-1]
}
