package logagents

import (
	"fmt"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/skolo/go/gcl"
	"go.skia.org/infra/skolo/go/logparser"
)

// rolloverLog watches two files, the primary log and the first rollover log (e.g. log.1)
// It hashes them every time it scans and uses that information to detect if any changes happened
// and/or if the logs rolled over.
type rolloverLog struct {
	reportName   string
	parse        logparser.Parser
	lastLine     int
	logPath      string
	logHash      string
	rolloverPath string
	rolloverHash string

	isFirstScan bool
}

func NewRollover(parse logparser.Parser, reportName, logFile, rolloverFile string) LogScanner {
	return &rolloverLog{
		reportName:   reportName,
		parse:        parse,
		logPath:      logFile,
		rolloverPath: rolloverFile,
		isFirstScan:  true,
	}
}
func (r *rolloverLog) ReportName() string {
	return r.reportName
}

func (r *rolloverLog) Scan(client gcl.CloudLogger) error {
	logC, logH, err := readAndHashFile(r.logPath)
	if err != nil {
		return fmt.Errorf("Problem reading log file %s: %s", r.logPath, err)
	}
	glog.Infof("Read log file with hash %s", logH)

	rollC, rollH, err := readAndHashFile(r.rolloverPath)
	if err != nil {
		return fmt.Errorf("Problem reading rollover file %s: %s", r.rolloverPath, err)
	}

	if r.isFirstScan {
		// This was the first scan and there was something already there.
		// We will not log that stuff now.
		r.rolloverHash = rollH
		r.isFirstScan = false
	}
	if r.rolloverHash != rollH {
		// We rolled over
		glog.Infof("Detected rollover to file %s", r.rolloverPath)
		r.rolloverHash = rollH
		lp := r.parse(rollC)
		r.reportLogs(client, lp, r.lastLine)
		r.lastLine = 0
	}

	if r.logHash != logH {
		r.logHash = logH
		lp := r.parse(logC)
		r.reportLogs(client, lp, r.lastLine)
		r.lastLine = lp.CurrLine()
	}
	// TODO(kjlubick) write to a text file in a future CL
	return nil
}

func (r *rolloverLog) reportLogs(client gcl.CloudLogger, lp logparser.ParsedLog, startLine int) {
	lp.Start(startLine)
	for log := lp.ReadAndNext(); log != nil; log = lp.ReadAndNext() {
		client.Log(r.reportName, log)
	}
}
