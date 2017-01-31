package deduplicator

import (
	"crypto/sha1"
	"fmt"
	"sync"

	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
)

type remoteDeduplicator struct {
	// seen maps gcs paths [including the hash of the keys] to true/false
	seen map[string]bool

	// revision is the Skia revision that the deduplicator remembers
	revision  string
	mutex     sync.Mutex
	gcsClient storage.FuzzerGCSClient
}

func NewRemoteDeduplicator(gcsClient storage.FuzzerGCSClient) Deduplicator {
	return &remoteDeduplicator{
		seen:      make(map[string]bool),
		gcsClient: gcsClient,
	}
}

func (d *remoteDeduplicator) SetRevision(r string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.revision = r
	// clear out everything we have seen thus far.
	d.seen = make(map[string]bool)
}

var FILE_WRITE_OPTS = gcs.FileWriteOptions{ContentEncoding: "text/plain"}

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
	gcsPath := fmt.Sprintf("%s/%s/%s/traces/%s", report.FuzzCategory, d.revision, report.FuzzArchitecture, k)

	if d.seen[gcsPath] {
		sklog.Debugf("%s has already been seen", gcsPath)
		return false
	}
	// cache it, because we've seen it now.
	d.seen[gcsPath] = true

	if _, err := d.gcsClient.GetFileContents(context.Background(), gcsPath); err != nil {
		// It doesn't exist, so we haven't seen it before.

		if err = d.gcsClient.SetFileContents(context.Background(), gcsPath, FILE_WRITE_OPTS, []byte(k)); err != nil {
			sklog.Warningf("Error while writing to deduplication %s", err)
		}

		return true
	}
	return false
}

func hash(s string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(s)))
}
