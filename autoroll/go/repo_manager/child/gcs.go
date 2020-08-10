package child

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"sort"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vfs"
	"google.golang.org/api/option"
)

var (
	// errInvalidGCSVersion is returned by gcsGetVersionFunc when a file in
	// GCS does not represent a valid revision.
	errInvalidGCSVersion = errors.New("Invalid GCS version.")
)

// GCSConfig provides configuration for a Child which reads from GCS.
type GCSConfig struct {
	// GCS bucket used for finding child revisions.
	GCSBucket string
	// Path within the GCS bucket which contains child revisions.
	GCSPath string
}

// See documentation for util.Validator interface.
func (c *GCSConfig) Validate() error {
	if c.GCSBucket == "" {
		return errors.New("GCSBucket is required.")
	}
	if c.GCSPath == "" {
		return errors.New("GCSPath is required.")
	}
	return nil
}

// gcsVersion represents a version of a file in GCS. It can be compared to other
// gcsVersion instances of the same type.
type gcsVersion interface {
	// Compare returns 0 if the given gcsVersion is equal to this one, >0 if
	// this gcsVersion comes before the given gcsVersion, and <0 if this
	// gcsVersion comes after the given gcsVersion.
	Compare(gcsVersion) int

	// Id returns the ID of this gcsVersion.
	Id() string
}

type gcsVersionSlice []gcsVersion

func (s gcsVersionSlice) Len() int {
	return len(s)
}

func (s gcsVersionSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// We sort newest to oldest.
func (s gcsVersionSlice) Less(i, j int) bool {
	return s[i].Compare(s[j]) < 0
}

// gcsGetVersionFunc is a function which returns a gcsVersion based on the given
// object in GCS. If the given object does not represent a valid gcsVersion, the
// func should return errInvalidGCSVersion, and the object will be ignored. Any
// other error will be logged and the object still ignored.
type gcsGetVersionFunc func(*revision.Revision) (gcsVersion, error)

// gcsShortRevFunc is a function which returns a shortened revision ID.
type gcsShortRevFunc func(string) string

// gcsChild is a Child implementation which loads revisions from GCS.
type gcsChild struct {
	gcs           *gcsclient.StorageClient
	gcsBucket     string
	gcsPath       string
	getGCSVersion gcsGetVersionFunc
	shortRev      gcsShortRevFunc
}

// newGCS returns a Child implementation which loads revision from GCS.
func newGCS(ctx context.Context, c GCSConfig, client *http.Client, getVersion gcsGetVersionFunc, shortRev gcsShortRevFunc) (*gcsChild, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	gcsClient := gcsclient.New(storageClient, c.GCSBucket)
	return &gcsChild{
		gcs:           gcsClient,
		gcsBucket:     c.GCSBucket,
		gcsPath:       c.GCSPath,
		getGCSVersion: getVersion,
		shortRev:      shortRev,
	}, nil
}

// See documentation for Child interface.
func (c *gcsChild) Update(ctx context.Context, lastRollRev *revision.Revision) (*revision.Revision, []*revision.Revision, error) {
	// Find the available versions, sorted newest to oldest.
	versions := []gcsVersion{}
	revisions := map[string]*revision.Revision{}
	if err := c.gcs.AllFilesInDirectory(ctx, c.gcsPath, func(item *storage.ObjectAttrs) {
		rev := c.objectAttrsToRevision(item)
		ver, err := c.getGCSVersion(rev)
		if err == nil {
			versions = append(versions, ver)
			revisions[rev.Id] = rev
		} else if err == errInvalidGCSVersion {
			// There are files we don't care about in this bucket. Just ignore.
		} else {
			sklog.Error(err)
		}
	}); err != nil {
		return nil, nil, err
	}
	if len(versions) == 0 {
		return nil, nil, fmt.Errorf("No valid files found in GCS.")
	}
	sort.Sort(gcsVersionSlice(versions))

	lastIdx := -1
	for idx, v := range versions {
		rev := revisions[v.Id()]
		if rev.Id == lastRollRev.Id {
			lastIdx = idx
			break
		}
	}
	if lastIdx == -1 {
		sklog.Errorf("Last roll rev %q not found in available versions. This is acceptable for some rollers which allow outside versions to be rolled manually (eg. AFDO roller). A human should verify that this is indeed caused by a manual roll. Using the single most recent available version for the not-yet-rolled revisions list, and attempting to retrieve the last-rolled rev. The revisions listed in the commit message will be incorrect!", lastRollRev.Id)
		lastIdx = 1
	}

	// Get the list of not-yet-rolled revisions.
	notRolledRevs := make([]*revision.Revision, 0, lastIdx)
	for i := 0; i < lastIdx; i++ {
		notRolledRevs = append(notRolledRevs, revisions[versions[i].Id()])
	}
	tipRev := lastRollRev
	if len(notRolledRevs) > 0 {
		tipRev = notRolledRevs[0]
	}
	return tipRev, notRolledRevs, nil
}

// See documentation for Child interface.
func (c *gcsChild) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	item, err := c.gcs.GetFileObjectAttrs(ctx, path.Join(c.gcsPath, id))
	if err != nil {
		return nil, err
	}
	return c.objectAttrsToRevision(item), nil
}

// VFS implements the Child interface.
func (c *gcsChild) VFS(ctx context.Context, rev *revision.Revision) (vfs.FS, error) {
	// VFS is not implemented for gcsChild, because we can't know whether the
	// target is an ordinary file or some type of archive which needs to be
	// extracted before being read. Note that we could implement this by having
	// the caller pass in an extraction function, but at the time of writing no
	// rollers which use gcsChild need VCS.
	return nil, skerr.Fmt("VFS not implemented for gcsChild")
}

// objectAttrsToRevision returns a revision.Revision based on the given
// storage.ObjectAttrs. It is intended to be used by structs which embed
// gcsChild as a helper for creating revision.Revisions.
func (c *gcsChild) objectAttrsToRevision(item *storage.ObjectAttrs) *revision.Revision {
	id := path.Base(item.Name)
	return &revision.Revision{
		Id:        id,
		Display:   c.shortRev(id),
		Author:    item.Owner,
		Timestamp: item.Updated,
		URL:       item.MediaLink,
	}
}

// gcsChild implements Child.
var _ Child = &gcsChild{}
