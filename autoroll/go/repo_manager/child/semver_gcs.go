package child

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/proto"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

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
	}
	return nil, errInvalidGCSVersion
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
		return nil, skerr.Wrap(err)
	}
	return &semVersion{
		id:      rev.Id,
		version: ver,
	}, nil
}

// ErrShortRevNoMatch is returned by ShortRev when the revision ID does not
// match the regular expression.
var ErrShortRevNoMatch = errors.New("Revision ID does not match provided regular expression")

// ShortRev shortens the revision ID using the given regular expression.
func ShortRev(reTmpl string, id string) (string, error) {
	shortRevRegex, err := regexp.Compile(reTmpl)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to compile ShortRevRegex")
	}
	matches := shortRevRegex.FindStringSubmatch(id)
	if len(matches) == 0 {
		// TODO(borenet): It'd be nice to log an error here to
		// indicate that the regex might be incorrect, but this
		// function may be called for revisions which are not
		// valid and thus may not match the regex. That would
		// cause an unhelpful error spew in the log.
		return "", ErrShortRevNoMatch
	} else if len(matches) == 1 {
		return matches[0], nil
	} else {
		// This indicates that there is at least one sub-match. We don't
		// have any way of combining multiple sub-matches into one short
		// revision, so just use the first one.
		return matches[1], nil
	}
}

// semVerShortRev shortens the revision ID using the given regular expression.
func semVerShortRev(reTmpl string, id string) string {
	shortRev, err := ShortRev(reTmpl, id)
	if err != nil {
		if err == ErrShortRevNoMatch {
			// TODO(borenet): It'd be nice to log an error here to
			// indicate that the regex might be incorrect, but this
			// function may be called for revisions which are not
			// valid and thus may not match the regex. That would
			// cause an unhelpful error spew in the log.
		} else {
			sklog.Error(err)
		}
		return id
	}
	return shortRev
}

// NewSemVerGCS returns a Child which uses semantic versioning to compare object
// versions in GCS.
func NewSemVerGCS(ctx context.Context, c *proto.SemVerGCSChildConfig, reg *config_vars.Registry, client *http.Client) (*gcsChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	versionRegex, err := config_vars.NewTemplate(c.VersionRegex)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := reg.Register(versionRegex); err != nil {
		return nil, skerr.Wrap(err)
	}
	var shortRevRegex *config_vars.Template
	if c.ShortRevRegex != "" {
		shortRevRegex, err = config_vars.NewTemplate(c.ShortRevRegex)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := reg.Register(shortRevRegex); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	getGCSVersion := func(rev *revision.Revision) (gcsVersion, error) {
		versionRegex, err := regexp.Compile(versionRegex.String())
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return getSemanticGCSVersion(versionRegex, rev)
	}
	shortRevFn := func(id string) string {
		if shortRevRegex != nil {
			return semVerShortRev(shortRevRegex.String(), id)
		}
		return id
	}
	return newGCS(ctx, c.Gcs, client, getGCSVersion, shortRevFn)
}
