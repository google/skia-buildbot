package logagents

import (
	"fmt"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/logparser"
)

// The raspberry pi might run out of memory if we don't log in smallish batches.
const BATCH_REPORT_SIZE = 100

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
		sklog.Warningf("Could not read from persistence file for %s.  Starting with default values: %s", reportName, err)
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
	sklog.Infof("Read %s with hash %s", r.LogPath, logH)

	rollC, rollH, err := readAndHashFile(r.RolloverPath)
	if err != nil {
		return fmt.Errorf("Problem reading rollover file %s: %s", r.RolloverPath, err)
	}

	if r.IsFirstScan {
		// This was the first scan and there was something already there.
		// We will not log that stuff now.
		r.RolloverHash = rollH

	}
	if r.RolloverHash != rollH {
		// We rolled over
		sklog.Infof("Detected rollover to file %s", r.RolloverPath)
		r.RolloverHash = rollH
		lp := r.Parse(rollC)
		r.reportLogs(client, lp, r.LastLine)
		r.LastLine = 0
	}

	if r.LogHash != logH {
		r.LogHash = logH
		lp := r.Parse(logC)
		start := r.LastLine
		if r.IsFirstScan && lp.Len() > 1000 {
			// Only log the first 1000 lines of a new log file, to avoid OOM problems.
			start = lp.Len() - 1000
		}
		sklog.Infof("Starting %s log at line %d out of %d", r.ReportName(), start, lp.Len())
		r.reportLogs(client, lp, start)
		r.LastLine = lp.CurrLine()
	}

	r.IsFirstScan = false
	sklog.Infof("Finished %s at line %d", r.ReportName(), r.LastLine)
	return writeToPersistenceFile(r.ReportName(), r)
}

func (r *rolloverLog) reportLogs(client sklog.CloudLogger, lp logparser.ParsedLog, startLine int) {
	lp.Start(startLine)
	logs := make([]*sklog.LogPayload, 0, BATCH_REPORT_SIZE)
	for log := lp.ReadAndNext(); log != nil; log = lp.ReadAndNext() {
		logs = append(logs, log)
		if len(logs) >= BATCH_REPORT_SIZE {
			client.BatchCloudLog(r.ReportName(), logs...)
			logs = nil
		}
	}
	client.BatchCloudLog(r.ReportName(), logs...)
}
