package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/util"
)

type SemVerGCSRepoManagerConfig struct {
	GCSRepoManagerConfig

	// ShortRevRegex is a regular expression string which indicates
	// what part of the revision ID string (ie. the file name in GCS)
	// should be used as the shortened ID for display. If not specified,
	// of if for some reason it does not match the revision ID, the full ID
	// string is used.
	ShortRevRegex string

	// VersionRegex is a regular expression string used for parsing version
	// numbers out of a file name in GCS. It should contain one or more
	// integer capture groups, which may or may not be named. The integers
	// matched by the capture groups are used when comparing two revisions,
	// according to the following rules:
	//
	// 1. Integers derived from named capture groups are compared first. The
	//    names are sorted (alphanumerically, ie. "11" comes before "2") and
	//    the integers are compared (as integers, not alphanumerically) in
	//    the resulting order.
	//
	// 2. Integers derived from un-named capture groups are compared next,
	//    in order of appearance.
	//
	// It is technically possible to mix named and un-named capture groups,
	// but it will almost certainly lead to confusion and thus it is
	// encouraged to use one or the other, preferring un-named groups unless
	// the comparison order needs to be changed.
	//
	// Examples:
	//  1. `(?P<group1>\d+).(?P<group2>\d+).(?P<group3>\d+)` is equivalent
	//     to `(\d+).(\d+).(\d+)`:
	//    a. "1.2.3" sorts before "1.2.4".
	//    b. "1.2.11" sorts after "1.2.3", because individual matches are
	//       compared as ints, not alphanumerically.
	//
	//  2. `(?P<group2>\d+) (?P<group1>\d+)`:
	//    a. "1 2" sorts after "2 1" because the second group name sorts
	//       before the first.
	//
	//  3. `(?P<9>\d+) (?P<10>\d+)`:
	//    a. "1 2" sorts after "2 1" because the second group name sorts
	//       before the first (alphanumeric sorting of group names).
	//
	//  4. `(\d+) (?P<group2>\d+) (?P<group1>\d+) (\d+)` (please don't):
	//    a. "1 2 3 4" sorts after "2 4 1 3" because the second named group
	//       sorts before the first named group, and un-named capture groups
	//       are compared after named groups.
	//    b. "1 2 3 4" sorts before "2 2 3 4".
	//    c. "2 2 3 4" sorts before "1 2 4 4".
	VersionRegex string
}

func (c *SemVerGCSRepoManagerConfig) Validate() error {
	if err := c.GCSRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.VersionRegex == "" {
		return errors.New("VersionRegex is required.")
	}
	return nil
}

// semanticVersion represents a version ID parsed from a string.
type semanticVersion struct {
	named   map[string]int
	unnamed []int
}

// parseSemanticVersion returns the set of integer versions which make up the
// given semantic version, as specified by the capture groups in the given
// Regexp.
func parseSemanticVersion(regex *regexp.Regexp, ver string) (*semanticVersion, error) {
	names := regex.SubexpNames()
	matches := regex.FindStringSubmatch(ver)
	if len(matches) > 1 {
		named := make(map[string]int, len(matches)-1)
		unnamed := make([]int, 0, len(matches)-1)
		for idx := 1; idx < len(matches); idx++ {
			v, err := strconv.Atoi(matches[idx])
			if err != nil {
				return nil, fmt.Errorf("Failed to parse int from regex match string; is the regex incorrect?")
			}
			name := names[idx]
			if name == "" {
				unnamed = append(unnamed, v)
			} else {
				named[name] = v
			}
		}
		return &semanticVersion{
			named:   named,
			unnamed: unnamed,
		}, nil
	} else {
		return nil, errInvalidGCSVersion
	}
}

// compareSemanticVersions returns 1 if A comes before B, -1 if A comes
// after B, and 0 if they are equal.
func compareSemanticVersions(a, b *semanticVersion) int {
	// Compare the named groups.
	nameSet := util.NewStringSet()
	for name := range a.named {
		nameSet[name] = true
	}
	for name := range b.named {
		nameSet[name] = true
	}
	names := nameSet.Keys()
	sort.Strings(names)
	for _, name := range names {
		if a.named[name] < b.named[name] {
			return 1
		} else if a.named[name] > b.named[name] {
			return -1
		}
	}

	// Compare the unnamed groups.
	for i := 0; ; i++ {
		if i == len(a.unnamed) && i == len(b.unnamed) {
			return 0
		} else if i == len(a.unnamed) {
			return 1
		} else if i == len(b.unnamed) {
			return -1
		}
		if a.unnamed[i] < b.unnamed[i] {
			return 1
		} else if a.unnamed[i] > b.unnamed[i] {
			return -1
		}
	}
}

// semVersion is an implementation of gcsVersion which uses semantic versioning.
type semVersion struct {
	id      string
	version *semanticVersion
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
func NewSemVerGCSRepoManager(ctx context.Context, c *SemVerGCSRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	versionRegex, err := regexp.Compile(c.VersionRegex)
	if err != nil {
		return nil, err
	}
	getGCSVersion := func(rev *revision.Revision) (gcsVersion, error) {
		return getSemanticGCSVersion(versionRegex, rev)
	}
	var shortRevRegex *regexp.Regexp
	if c.ShortRevRegex != "" {
		shortRevRegex, err = regexp.Compile(c.ShortRevRegex)
		if err != nil {
			return nil, err
		}
	}
	shortRev := func(id string) string {
		if shortRevRegex != nil {
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
	return newGCSRepoManager(ctx, &c.GCSRepoManagerConfig, workdir, g, serverURL, gitcookiesPath, client, cr, local, getGCSVersion, shortRev)
}
