package fileutil

import (
	"os"
	"path/filepath"

	"github.com/golang/glog"
)

// EnsureDirExists checks whether the given path to a directory exits and creates it
// if necessary. Returns the absolute path that corresponds to the input path
// and an error indicating a problem.
func EnsureDirExists(dirPath string) (string, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}

	return absPath, os.MkdirAll(absPath, 0700)
}

// Must checks whether err in the provided pair (s, err) is nil. If so it
// returns s otherwise it cause the program to stop with the error message.
func Must(s string, err error) string {
	if err != nil {
		glog.Fatal(err)
	}
	return s
}
