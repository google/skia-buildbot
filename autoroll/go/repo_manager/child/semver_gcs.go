package child

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/semver"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

type gcsSemVersion semver.Version

func (v *gcsSemVersion) Compare(other gcsVersion) int {
	return (*semver.Version)(v).Compare((*semver.Version)(other.(*gcsSemVersion)))
}

func (v *gcsSemVersion) Id() string {
	return (*semver.Version)(v).String()
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
func NewSemVerGCS(ctx context.Context, c *config.SemVerGCSChildConfig, client *http.Client) (*gcsChild, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	parser, err := semver.NewParser(c.VersionRegex)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	getGCSVersion := func(rev *revision.Revision) (gcsVersion, error) {
		semVer, err := parser.Parse(rev.Id)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return (*gcsSemVersion)(semVer), nil
	}
	getGCSPrefix := func() (string, error) {
		// LiteralPrefix gives the literal string before any special regex
		// characters. The leading caret obscures the prefix we want to find.
		versionRegexStr := strings.TrimPrefix(c.VersionRegex, "^")
		versionRegex, err := regexp.Compile(versionRegexStr)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		prefix, _ := versionRegex.LiteralPrefix()
		sklog.Infof("Derived GCS search prefix %q from regex %q", prefix, versionRegex.String())
		return prefix, nil
	}
	shortRevFn := func(id string) string {
		if c.ShortRevRegex != "" {
			return semVerShortRev(c.ShortRevRegex, id)
		}
		return id
	}
	return newGCS(ctx, c.Gcs, client, getGCSVersion, shortRevFn, getGCSPrefix)
}
