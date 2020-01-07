package repo_manager

/*
   RepoManager which rolls based on files in GCS.
*/

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

var (
	errInvalidGCSVersion = errors.New("Invalid GCS version.")
)

type GCSRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig

	// GCS bucket used for finding child revisions.
	GCSBucket string
	// Path within the GCS bucket which contains child revisions.
	GCSPath string
	// File to update in the parent repo.
	VersionFile string
}

func (c *GCSRepoManagerConfig) Validate() error {
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.GCSBucket == "" {
		return errors.New("GCSBucket is required.")
	}
	if c.GCSPath == "" {
		return errors.New("GCSPath is required.")
	}
	if c.VersionFile == "" {
		return errors.New("VersionFile is required.")
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

// getGCSVersionFunc is a function which returns a gcsVersion based on the given
// object in GCS. If the given object does not represent a valid gcsVersion, the
// func should return errInvalidGCSVersion, and the object will be ignored. Any
// other error will be logged and the object still ignored.
type getGCSVersionFunc func(*revision.Revision) (gcsVersion, error)

// shortRevFunc is a function which returns a shortened revision ID.
type shortRevFunc func(string) string

// gcsRepoManager is a RepoManager which creates rolls based on files in GCS.
type gcsRepoManager struct {
	*noCheckoutRepoManager
	gcs           gcs.GCSClient
	gcsBucket     string
	gcsPath       string
	getGCSVersion getGCSVersionFunc
	shortRev      shortRevFunc
	versionFile   string
}

// Return a gcsRepoManager instance.
func newGCSRepoManager(ctx context.Context, c *GCSRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL string, client *http.Client, cr codereview.CodeReview, local bool, getGCSVersion getGCSVersionFunc, shortRev shortRevFunc) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	gcsClient := gcsclient.New(storageClient, c.GCSBucket)
	rv := &gcsRepoManager{
		gcs:           gcsClient,
		gcsBucket:     c.GCSBucket,
		gcsPath:       c.GCSPath,
		getGCSVersion: getGCSVersion,
		shortRev:      shortRev,
		versionFile:   c.VersionFile,
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, workdir, g, serverURL, client, cr, rv.createRoll, rv.updateHelper, local)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *gcsRepoManager) createRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, serverURL, cqExtraTrybots string, emails []string, baseCommit string) (string, map[string]string, error) {
	commitMsg, err := rm.buildCommitMsg(&CommitMsgVars{
		CqExtraTrybots: cqExtraTrybots,
		Reviewers:      emails,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      serverURL,
	})
	if err != nil {
		return "", nil, err
	}
	return commitMsg, map[string]string{rm.versionFile: to.Id}, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *gcsRepoManager) updateHelper(ctx context.Context, parentRepo *gitiles.Repo, baseCommit string) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Read the version file to determine the last roll rev.
	buf := bytes.NewBuffer([]byte{})
	if err := parentRepo.ReadFileAtRef(ctx, rm.versionFile, baseCommit, buf); err != nil {
		return nil, nil, nil, err
	}
	lastRollRevId := strings.TrimSpace(buf.String())

	// Find the available versions, sorted newest to oldest.
	versions := []gcsVersion{}
	revisions := map[string]*revision.Revision{}
	if err := rm.gcs.AllFilesInDirectory(ctx, rm.gcsPath, func(item *storage.ObjectAttrs) {
		rev := rm.objectAttrsToRevision(item)
		ver, err := rm.getGCSVersion(rev)
		if err == nil {
			versions = append(versions, ver)
			revisions[rev.Id] = rev
		} else if err == errInvalidGCSVersion {
			// There are files we don't care about in this bucket. Just ignore.
		} else {
			sklog.Error(err)
		}
	}); err != nil {
		return nil, nil, nil, err
	}
	if len(versions) == 0 {
		return nil, nil, nil, fmt.Errorf("No valid files found in GCS.")
	}
	sort.Sort(gcsVersionSlice(versions))

	lastIdx := -1
	var lastRollRev *revision.Revision
	for idx, v := range versions {
		rev := revisions[v.Id()]
		if rev.Id == lastRollRevId {
			lastIdx = idx
			lastRollRev = rev
			break
		}
	}
	if lastIdx == -1 {
		sklog.Errorf("Last roll rev %q not found in available versions. This is acceptable for some rollers which allow outside versions to be rolled manually (eg. AFDO roller). A human should verify that this is indeed caused by a manual roll. Using the single most recent available version for the not-yet-rolled revisions list, and attempting to retrieve the last-rolled rev. The revisions listed in the commit message will be incorrect!", lastRollRevId)
		lastIdx = 1

		lastRevPath := path.Join(rm.gcsPath, lastRollRevId)
		item, err := rm.gcs.GetFileObjectAttrs(ctx, lastRevPath)
		if err != nil {
			sklog.Errorf("Failed to retrieve last roll rev at %s; creating incomplete Revision: %s", lastRevPath, err)
			lastRollRev = &revision.Revision{
				Id:      lastRollRevId,
				Display: rm.shortRev(lastRollRevId),
			}
		} else {
			lastRollRev = rm.objectAttrsToRevision(item)
		}
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
	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (r *gcsRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	item, err := r.gcs.GetFileObjectAttrs(ctx, path.Join(r.gcsPath, id))
	if err != nil {
		return nil, err
	}
	return r.objectAttrsToRevision(item), nil
}

// objectAttrsToRevision returns a revision.Revision based on the given
// storage.ObjectAttrs. It is intended to be used by structs which embed
// gcsRepoManager as a helper for creating revision.Revisions.
func (r *gcsRepoManager) objectAttrsToRevision(item *storage.ObjectAttrs) *revision.Revision {
	id := path.Base(item.Name)
	return &revision.Revision{
		Id:        id,
		Display:   r.shortRev(id),
		Author:    item.Owner,
		Timestamp: item.Updated,
		URL:       item.MediaLink,
	}
}
