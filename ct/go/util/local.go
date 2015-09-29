// Utilities for running on a local machine.
package util

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/skia-dev/glog"
)

func SetVarsForLocal() {
	CtAdmins = nil
	CtUser = ""
	if u, err := user.Current(); err == nil {
		CtAdmins = []string{u.Username + "@google.com"}
		CtUser = u.Username
	}
	Slaves = []string{"127.0.0.1"}
	if h, err := os.Hostname(); err == nil {
		Slaves = []string{h}
	}
	_, currentFile, _, _ := runtime.Caller(0)
	myPathToCt := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
	realMyPathToCt, err1 := filepath.EvalSymlinks(myPathToCt)
	realCtTreeDir, err2 := filepath.EvalSymlinks(CtTreeDir)
	if err1 == nil && err2 == nil && realMyPathToCt != realCtTreeDir {
		glog.Fatalf("Master and worker scripts believe CT tree is at %s, but it appears to actually be at %s. Did you set up a symlink?", realCtTreeDir, realMyPathToCt)
	}
	GSBucketName = "cluster-telemetry-test"
}
