package revision_filter

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/skerr"
)

// CIPDRevisionFilter is a RevisionFilter which filters out Revisions which
// don't exist on all of the configured packages and platforms. Only works with
// rollers who use a tag as the Revision ID, eg. git_revision.
type CIPDRevisionFilter struct {
	client    cipd.CIPDClient
	packages  []string
	platforms []string
}

// Skip implements RevisionFilter.
func (f *CIPDRevisionFilter) Skip(ctx context.Context, r *revision.Revision) (string, error) {
	if len(strings.Split(r.Id, ":")) != 2 {
		return fmt.Sprintf("Revision ID %q doesn't follow CIPD tag format", r.Id), nil
	}
	for _, pkg := range f.packages {
		for _, platform := range f.platforms {
			pkgFullPath := pkg + "/" + platform
			// Note that this only works for rollers which use a tag as the
			// Revision ID.  If the revision ID is the package ID, it is likely
			// that this request will fail because the ID doesn't follow the
			// expected "key:value" tag format, and if it doesn't fail it will
			// only ever return an empty set of results because the package ID
			// isn't a tag.
			pins, err := f.client.SearchInstances(ctx, pkgFullPath, []string{r.Id})
			if err != nil {
				return "", skerr.Wrap(err)
			}
			if len(pins) == 0 {
				return fmt.Sprintf("CIPD package %q does not exist at revision %q", pkgFullPath, r.Id), nil
			}
		}
	}
	return "", nil
}

// NewCIPDRevisionFilter returns a RevisionFilter which filters out Revisions
// which don't exist on all of the configured packages and platforms.
func NewCIPDRevisionFilter(client *http.Client, cfg *config.CIPDRevisionFilterConfig, workdir string) (*CIPDRevisionFilter, error) {
	cipdClient, err := cipd.NewClient(client, workdir, cipd.DefaultServiceURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &CIPDRevisionFilter{
		client:    cipdClient,
		packages:  cfg.Package,
		platforms: cfg.Platform,
	}, nil
}

var _ RevisionFilter = &CIPDRevisionFilter{}
