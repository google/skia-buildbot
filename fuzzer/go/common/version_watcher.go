package common

import (
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/metrics2"
)

// A VersionHandler is the type of the callbacks used by VersionWatcher.
type VersionHandler func(string) error

// VersionWatcher handles the logic to wait for the version under fuzz to change in
// Google Storage.  When it notices the pending version change or the current version
// change, it will fire off one of two provided callbacks.
// It also provides a way for clients to access the current and pending versions seen
// by this watcher.
// The callbacks are not fired on the initial values of the versions, only when a change
// happens.
type VersionWatcher struct {
	// A channel to monitor any fatal errors encountered by the version watcher.
	Status chan error

	storageClient   *storage.Client
	polingPeriod    time.Duration
	LastCurrentHash string
	LastPendingHash string
	onPendingChange VersionHandler
	onCurrentChange VersionHandler
	force           chan bool
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
		force:           make(chan bool),
	}
}

// Start starts a goroutine that will occasionally wake up (as specified by the period)
// and check to see if the current or pending skia versions to fuzz have changed.
func (vw *VersionWatcher) Start() {
	go func() {
		t := time.Tick(vw.polingPeriod)
		for {
			select {
			case <-vw.force:
				vw.check()
			case <-t:
				vw.check()
			}
		}
	}()
}

// check looks in Google Storage to see if the pending or current versions have updated. If so, it
// synchronously calls the relevent callbacks and updates this objects LastCurrentHash
// and/or LastPendingHash.
func (vw *VersionWatcher) check() {
	glog.Infof("Woke up to check the Skia version, last current seen was %s", vw.LastCurrentHash)

	current, lastUpdated, err := GetCurrentSkiaVersionFromGCS(vw.storageClient)
	if err != nil {
		glog.Errorf("Failed getting current Skia version from GCS.  Going to try again: %s", err)
		return
	}
	glog.Infof("Current version found to be %q, updated at %v", current, lastUpdated)
	if vw.LastCurrentHash == "" {
		vw.LastCurrentHash = current
	} else if current != vw.LastCurrentHash && vw.onCurrentChange != nil {
		glog.Infof("Calling onCurrentChange(%q)", current)
		if err := vw.onCurrentChange(current); err != nil {
			vw.Status <- fmt.Errorf("Failed while executing onCurrentChange %#v.We could be in a broken state. %s", vw.onCurrentChange, err)
			return
		}
		vw.LastCurrentHash = current
		lastUpdated = time.Now()
	}

	if !lastUpdated.IsZero() {
		metrics2.GetInt64Metric("fuzzer.version.age", map[string]string{"type": "current"}).Update(int64(time.Since(lastUpdated) / time.Second))
	}

	if config.Common.ForceReanalysis {
		if err := vw.onPendingChange(vw.LastCurrentHash); err != nil {
			glog.Errorf("There was a problem during force analysis: %s", err)
		}
		config.Common.ForceReanalysis = false
		return
	}

	pending, lastUpdated, err := GetPendingSkiaVersionFromGCS(vw.storageClient)
	if err != nil {
		glog.Errorf("Failed getting pending Skia version from GCS.  Going to try again: %s", err)
		return
	}
	glog.Infof("Pending version found to be %q, updated at %v", pending, lastUpdated)
	if pending == "" {
		vw.LastPendingHash = ""
		lastUpdated = time.Now()
	} else if vw.LastPendingHash != pending {
		vw.LastPendingHash = pending
		lastUpdated = time.Now()

		if vw.onPendingChange != nil {
			glog.Infof("Calling onPendingChange(%q)", pending)
			if err := vw.onPendingChange(pending); err != nil {
				vw.Status <- fmt.Errorf("Failed while executing onCurrentChange %#v.We could be in a broken state. %s", vw.onCurrentChange, err)
				return
			}
		}
	}

	if !lastUpdated.IsZero() {
		metrics2.GetInt64Metric("fuzzer.version.age", map[string]string{"type": "pending"}).Update(int64(time.Since(lastUpdated) / time.Second))
	}
}

// Recheck forces a recheck of the pending and current version.
func (vw *VersionWatcher) Recheck() {
	vw.force <- true
}
