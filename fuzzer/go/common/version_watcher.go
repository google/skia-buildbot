package common

import (
	"fmt"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/vcsinfo"
	"google.golang.org/cloud/storage"
)

// A VersionHandler is the type of the callbacks used by VersionWatcher.
// The callbacks should return the vcsinfo.LongCommit of the new version
// to be used by other dependents on this VersionHandler.
type VersionHandler func(string) (*vcsinfo.LongCommit, error)

// VersionWatcher handles the logic to wait for the version under fuzz to change in
// Google Storage.  When it notices the pending version change or the current version
// change, it will fire off one of two provided callbacks.
// It also provides a way for clients to access the current and pending versions seen
// by this watcher.
// The callbacks are not fired on the initial values of the versions, only when a change
// happens.
type VersionWatcher struct {
	// The most recently changed current version.
	CurrentVersion *vcsinfo.LongCommit
	// The most recently changed pending version.
	PendingVersion *vcsinfo.LongCommit
	// A channel to monitor any fatal errors encountered by the version watcher.
	Status chan error

	storageClient   *storage.Client
	polingPeriod    time.Duration
	lastCurrentHash string
	lastPendingHash string
	onPendingChange VersionHandler
	onCurrentChange VersionHandler
}

// NewVersionWatcher creates a version watcher with the specified time period and access
// to GCS.  The supplied callbacks may be nil.
func NewVersionWatcher(s *storage.Client, period time.Duration, onPendingChange, onCurrentChange VersionHandler) *VersionWatcher {
	return &VersionWatcher{
		storageClient:   s,
		polingPeriod:    period,
		onPendingChange: onPendingChange,
		onCurrentChange: onCurrentChange,
		Status:          make(chan error),
	}
}

// Start starts a goroutine that will occasionally wake up (as specified by the period)
// and check to see if the current or pending skia versions to fuzz have changed.
// If so, it synchronously calls the relevent callbacks and updates this objects
// CurrentVersion and/or PendingVersion.
func (vw *VersionWatcher) Start() {
	go func() {
		t := time.Tick(vw.polingPeriod)
		for _ = range t {
			glog.Infof("Woke up to check the Skia version, last current seen was %s", vw.lastCurrentHash)

			current, lastUpdated, err := GetCurrentSkiaVersionFromGCS(vw.storageClient)
			if err != nil {
				glog.Errorf("Failed getting current Skia version from GCS.  Going to try again: %s", err)
				continue
			}
			glog.Infof("Current version found to be %q, updated at %v", current, lastUpdated)
			if vw.lastCurrentHash == "" {
				vw.lastCurrentHash = current
			} else if current != vw.lastCurrentHash && vw.onCurrentChange != nil {
				glog.Infof("Calling onCurrentChange(%q)", current)
				cv, err := vw.onCurrentChange(current)
				if err != nil {
					vw.Status <- fmt.Errorf("Failed while executing onCurrentChange %#v.We could be in a broken state. %s", vw.onCurrentChange, err)
					return
				}
				vw.CurrentVersion = cv
				vw.lastCurrentHash = current
				lastUpdated = time.Now()
			}

			if !lastUpdated.IsZero() {
				metrics2.GetInt64Metric("fuzzer.version.age", map[string]string{"type": "current"}).Update(int64(time.Since(lastUpdated) / time.Second))
			}

			if config.Common.ForceReanalysis {
				if _, err := vw.onPendingChange(vw.lastCurrentHash); err != nil {
					glog.Errorf("There was a problem during force analysis: %s", err)
				}
				config.Common.ForceReanalysis = false
				return
			}

			pending, lastUpdated, err := GetPendingSkiaVersionFromGCS(vw.storageClient)
			if err != nil {
				glog.Errorf("Failed getting pending Skia version from GCS.  Going to try again: %s", err)
				continue
			}
			glog.Infof("Pending version found to be %q, updated at %v", pending, lastUpdated)
			if pending == "" {
				vw.lastPendingHash = ""
				vw.PendingVersion = nil
				lastUpdated = time.Now()
			} else if vw.lastPendingHash != pending && vw.onPendingChange != nil {
				glog.Infof("Calling onPendingChange(%q)", pending)
				pv, err := vw.onPendingChange(pending)
				if err != nil {
					vw.Status <- fmt.Errorf("Failed while executing onCurrentChange %#v.We could be in a broken state. %s", vw.onCurrentChange, err)
					return
				}
				vw.PendingVersion = pv
				vw.lastPendingHash = pending
				lastUpdated = time.Now()
			}

			if !lastUpdated.IsZero() {
				metrics2.GetInt64Metric("fuzzer.version.age", map[string]string{"type": "pending"}).Update(int64(time.Since(lastUpdated) / time.Second))
			}
		}
	}()
}
