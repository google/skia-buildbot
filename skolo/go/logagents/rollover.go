package logagents

import (
	"fmt"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/logparser"
)

// rolloverLog watches two files, the primary log and the first rollover log (e.g. log.1)
// It hashes them every time it scans and uses that information to detect if any changes happened
// and/or if the logs rolled over.
type rolloverLog struct {
	Name         string
	Parse        logparser.Parser `json:"-"`
	LastLine     int
	LogPath      string
	LogHash      string
	RolloverPath string
	RolloverHash string

	IsFirstScan bool
}

func NewRollover(parse logparser.Parser, reportName, logFile, rolloverFile string) LogScanner {
	log := &rolloverLog{
		Name:         reportName,
		Parse:        parse,
		LogPath:      logFile,
		RolloverPath: rolloverFile,
		IsFirstScan:  true,
	}
	if err := readFromPersistenceFile(reportName, log); err != nil {
		glog.Warningf("Could not read from persistence file for %s.  Starting with default values: %s", reportName, err)
	}
	return log
}

func (r *rolloverLog) ReportName() string {
	return r.Name
}

func (r *rolloverLog) Scan(client sklog.CloudLogger) error {
	logC, logH, err := readAndHashFile(r.LogPath)
	if err != nil {
		return fmt.Errorf("Problem reading log file %s: %s", r.LogPath, err)
	}
	glog.Infof("Read log file with hash %s", logH)

	rollC, rollH, err := readAndHashFile(r.RolloverPath)
	if err != nil {
		return fmt.Errorf("Problem reading rollover file %s: %s", r.RolloverPath, err)
	}

	if r.IsFirstScan {
		// This was the first scan and there was something already there.
		// We will not log that stuff now.
		r.RolloverHash = rollH
		r.IsFirstScan = false
	}
	if r.RolloverHash != rollH {
		// We rolled over
		glog.Infof("Detected rollover to file %s", r.RolloverPath)
		r.RolloverHash = rollH
		lp := r.Parse(rollC)
		r.reportLogs(client, lp, r.LastLine)
		r.LastLine = 0
	}

	if r.LogHash != logH {
		r.LogHash = logH
		lp := r.Parse(logC)
		r.reportLogs(client, lp, r.LastLine)
		r.LastLine = lp.CurrLine()
	}
	return writeToPersistenceFile(r.ReportName(), r)
}

func (r *rolloverLog) reportLogs(client sklog.CloudLogger, lp logparser.ParsedLog, startLine int) {
	lp.Start(startLine)
	for log := lp.ReadAndNext(); log != nil; log = lp.ReadAndNext() {
		client.CloudLog(r.ReportName(), log)
	}
}
