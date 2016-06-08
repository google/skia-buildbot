package logagents

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/skolo/go/gcl"
)

// A LogScanner scans some log file(s), parses them, and reports any new logs to
// the specified CloudLogger.  ReportName() should be the name of the log as shared with GCL.
type LogScanner interface {
	Scan(logsclient gcl.CloudLogger) error
	ReportName() string
}

// readAndHashFile opens a file, reads the contents, hashes them and returns the contents and hash.
// This is a package level var so it can be swapped out when unit testing
var readAndHashFile = func(path string) (contents, hash string, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty string if file doesn't exist.  Same as "no logs".
			return "", "", nil
		}
		return "", "", fmt.Errorf("Problem opening log file %s: %s", path, err)
	}
	defer util.Close(f)
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", "", fmt.Errorf("Problem reading log file %s: %s", path, err)
	}

	return string(b), fmt.Sprintf("%x", sha1.Sum(b)), nil
}
