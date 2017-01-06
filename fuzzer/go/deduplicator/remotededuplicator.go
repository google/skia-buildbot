package deduplicator

import (
	"crypto/sha1"
	"fmt"
	"sync"

	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

type GCSFileGetter interface {
	GetFileContents(bucket, path string) ([]byte, error)
}

type GCSFileSetter interface {
	SetFileContents(bucket, path string, contents []byte) error
}

type RemoteDeduplicatorGCSClient interface {
	GCSFileGetter
	GCSFileSetter
}

type remoteDeduplicator struct {
	// maps gcs paths [including the hash of the keys] to true/false
	seen map[string]bool

	commit    string
	mutex     sync.Mutex
	gcsClient RemoteDeduplicatorGCSClient
}

func NewRemoteDeduplicator(gcsClient RemoteDeduplicatorGCSClient) Deduplicator {
	return &remoteDeduplicator{
		seen:      make(map[string]bool),
		gcsClient: gcsClient,
	}
}

func (d *remoteDeduplicator) Clear() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.seen = make(map[string]bool)
}

func (d *remoteDeduplicator) SetCommit(c string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.commit = c
}

func (d *remoteDeduplicator) IsUnique(report data.FuzzReport) bool {
	// Empty stacktraces should be manually deduplicated.
	if report.DebugStackTrace.IsEmpty() && report.ReleaseStackTrace.IsEmpty() {
		return true
	}
	// Other flags should also be looked at manually.
	if util.In("Other", report.DebugFlags) || util.In("Other", report.ReleaseFlags) {
		return true
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	k := hash(key(report))
	gcsPath := fmt.Sprintf("%s/%s/%s/traces/%s", report.FuzzCategory, d.commit, report.FuzzArchitecture, k)

	if d.seen[gcsPath] {
		return false
	}

	if _, err := d.gcsClient.GetFileContents(config.GS.Bucket, gcsPath); err != nil {
		// It doesn't exist, so we haven't seen it before.
		// cache it
		d.seen[gcsPath] = true
		if err = d.gcsClient.SetFileContents(config.GS.Bucket, gcsPath, []byte(k)); err != nil {
			sklog.Warningf("Error while writing to deduplication %s", err)
		}

		return true
	}
	return false
}

func hash(s string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(s)))
}
