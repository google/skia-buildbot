package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
)

type SemVerGCSRepoManagerConfig struct {
	GCSRepoManagerConfig

	// ShortRevRegex is a regular expression string which indicates
	// what part of the revision ID string should be used as the shortened
	// ID for display. If not specified, the full ID string is used.
	ShortRevRegex *config_vars.Template

	// VersionRegex is a regular expression string containing one or more
	// integer capture groups. The integers matched by the capture groups
	// are compared, in order, when comparing two revisions.
	VersionRegex *config_vars.Template
}

func (c *SemVerGCSRepoManagerConfig) Validate() error {
	if err := c.GCSRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.VersionRegex == nil {
		return errors.New("VersionRegex is required.")
	}
	if err := c.VersionRegex.Validate(); err != nil {
		return err
	}
	if c.ShortRevRegex != nil {
		if err := c.ShortRevRegex.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// parseSemanticVersion returns the set of integer versions which make up the
// given semantic version, as specified by the capture groups in the given
// Regexp.
func parseSemanticVersion(regex *regexp.Regexp, ver string) ([]int, error) {
	matches := regex.FindStringSubmatch(ver)
	if len(matches) > 1 {
		matchInts := make([]int, len(matches)-1)
		for idx, a := range matches[1:] {
			i, err := strconv.Atoi(a)
			if err != nil {
				return matchInts, fmt.Errorf("Failed to parse int from regex match string; is the regex incorrect?")
			}
			matchInts[idx] = i
		}
		return matchInts, nil
	} else {
		return nil, errInvalidGCSVersion
	}
}

// compareSemanticVersions returns 1 if A comes before B, -1 if A comes
// after B, and 0 if they are equal.
func compareSemanticVersions(a, b []int) int {
	for i := 0; ; i++ {
		if i == len(a) && i == len(b) {
			return 0
		} else if i == len(a) {
			return 1
		} else if i == len(b) {
			return -1
		}
		if a[i] < b[i] {
			return 1
		} else if a[i] > b[i] {
			return -1
		}
	}
}

// semVersion is an implementation of gcsVersion which uses semantic versioning.
type semVersion struct {
	id      string
	version []int
}

// See documentation for gcsVersion interface.
func (v *semVersion) Compare(other gcsVersion) int {
	a := v.version
	b := other.(*semVersion).version
	return compareSemanticVersions(a, b)
}

// See documentation for gcsVersion interface.
func (v *semVersion) Id() string {
	return v.id
}

// See documentation for getGCSVersionFunc.
func getSemanticGCSVersion(regex *regexp.Regexp, rev *revision.Revision) (gcsVersion, error) {
	ver, err := parseSemanticVersion(regex, rev.Id)
	if err != nil {
		return nil, err
	}
	return &semVersion{
		id:      rev.Id,
		version: ver,
	}, nil
}

// NewSemVerGCSRepoManager returns a gcsRepoManager which uses semantic
// versioning to compare object versions.
func NewSemVerGCSRepoManager(ctx context.Context, c *SemVerGCSRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := reg.Register(c.VersionRegex); err != nil {
		return nil, err
	}
	if c.ShortRevRegex != nil {
		if err := reg.Register(c.ShortRevRegex); err != nil {
			return nil, err
		}
	}
	getGCSVersion := func(rev *revision.Revision) (gcsVersion, error) {
		versionRegex, err := regexp.Compile(c.VersionRegex.String())
		if err != nil {
			return nil, err
		}
		return getSemanticGCSVersion(versionRegex, rev)
	}
	shortRev := func(id string) string {
		if c.ShortRevRegex != nil {
			shortRevRegex, err := regexp.Compile(c.ShortRevRegex.String())
			if err != nil {
				sklog.Errorf("Failed to compile c.ShortRevRegex: %s", err)
				return id
			}
			matches := shortRevRegex.FindStringSubmatch(id)
			if len(matches) > 0 {
				return matches[0]
			}
			// TODO(borenet): It'd be nice to log an error here to
			// indicate that the regex might be incorrect, but this
			// function may be called for revisions which are not
			// valid and thus may not match the regex. That would
			// cause an unhelpful error spew in the log.
		}
		return id
	}
	return newGCSRepoManager(ctx, &c.GCSRepoManagerConfig, reg, workdir, g, serverURL, client, cr, local, getGCSVersion, shortRev)
}
