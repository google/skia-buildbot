package revision_filter

import (
	"context"
	"fmt"

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
	tagKey    string
}

// Skip implements RevisionFilter.
func (f *CIPDRevisionFilter) Skip(ctx context.Context, r revision.Revision) (string, error) {
	tag := r.Id
	if f.tagKey != "" {
		tag = cipd.JoinTag(f.tagKey, tag)
	}
	if _, _, err := cipd.SplitTag(tag); err != nil {
		return fmt.Sprintf("%q doesn't follow CIPD tag format", tag), nil
	}
	var fullPathsToCheck []string
	for _, pkg := range f.packages {
		if len(f.platforms) == 0 {
			fullPathsToCheck = append(fullPathsToCheck, pkg)
		} else {
			for _, platform := range f.platforms {
				fullPathsToCheck = append(fullPathsToCheck, pkg+"/"+platform)
			}
		}
	}
	for _, pkgFullPath := range fullPathsToCheck {
		// Note that this only works for rollers which use a tag as the
		// Revision ID.  If the revision ID is the package ID, it is likely
		// that this request will fail because the ID doesn't follow the
		// expected "key:value" tag format, and if it doesn't fail it will
		// only ever return an empty set of results because the package ID
		// isn't a tag.
		pins, err := f.client.SearchInstances(ctx, pkgFullPath, []string{tag})
		if err != nil {
			return "", skerr.Wrap(err)
		}
		if len(pins) == 0 {
			return fmt.Sprintf("CIPD package %q does not exist at tag %q", pkgFullPath, tag), nil
		}
	}
	return "", nil
}

// Update implements RevisionFilter.
func (f *CIPDRevisionFilter) Update(_ context.Context) error {
	return nil
}

// NewCIPDRevisionFilter returns a RevisionFilter which filters out Revisions
// which don't exist on all of the configured packages and platforms.
func NewCIPDRevisionFilter(cipdClient cipd.CIPDClient, cfg *config.CIPDRevisionFilterConfig) (*CIPDRevisionFilter, error) {
	return &CIPDRevisionFilter{
		client:    cipdClient,
		packages:  cfg.Package,
		platforms: cfg.Platform,
		tagKey:    cfg.TagKey,
	}, nil
}

var _ RevisionFilter = &CIPDRevisionFilter{}
