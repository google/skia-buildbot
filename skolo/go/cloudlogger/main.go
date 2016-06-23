package main

import (
	"flag"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/skolo/go/gcl"
	"go.skia.org/infra/skolo/go/logagents"
	"go.skia.org/infra/skolo/go/logparser"
	logging "google.golang.org/api/logging/v2beta1"
)

var (
	rolloverLogs   = common.NewMultiStringFlag("rollover_logs", nil, "A set of log file paths that may roll over.  e.g. supply run_isolated.log to monitor run_isolated.log and run_isolated.log.1")
	persistenceDir = flag.String("persistence_dir", "/var/cloudlogger", "The directory in which persistence data regarding the logging progress should be kept.")

	pollPeriod = flag.Duration("poll_period", 1*time.Minute, `The period used to poll the log files`)
)

func main() {
	defer common.LogPanic()
	common.Init()

	client, err := auth.NewDefaultJWTServiceAccountClient(logging.LoggingWriteScope)
	if err != nil {
		gcl.Fatalf("Failed to create authenticated HTTP client: %s\nDid you run get_service_account?", err)
	}

	err = gcl.Init(client, "raspberry-pis", "cloudlogger")
	if err != nil {
		gcl.Fatalf("Problem creating logs service: %s", err)
	}
	if err := logagents.SetPersistenceDir(*persistenceDir); err != nil {
		gcl.Fatalf("Could not set Persistence Dir: %s", err)
	}

	scanners := []logagents.LogScanner{}
	for _, r := range *rolloverLogs {
		scanners = append(scanners, logagents.NewRollover(logparser.ParsePythonLog, cleanupName(r), r, r+".1"))
	}

	scan(scanners, *pollPeriod)
}

// scan executes Scan on all LogScanners on a repeating time clock of period.  This executes indefinitely.
func scan(scanners []logagents.LogScanner, period time.Duration) {
	for range time.Tick(period) {
		gcl.Infof("Waking up to scan logs.")
		for _, s := range scanners {
			gcl.Infof("Scanning %s", s.ReportName())
			if err := s.Scan(gcl.Instance()); err != nil {
				gcl.Errorf("Problem with log file %s : %s", s.ReportName(), err)
			}
		}
	}
}

// cleanupName removes .log from the end of the given name if it is there.
func cleanupName(s string) string {
	return strings.Replace(s, ".log", "", -1)
}
