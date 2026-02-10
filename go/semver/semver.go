package semver

import (
	"errors"
	"regexp"
	"strconv"

	"go.skia.org/infra/go/skerr"
)

// ErrNoMatch is returned when a version string does not match the regex.
var ErrNoMatch = errors.New("version string does not match regular expression")

// Version represents a semantic version.
type Version struct {
	asString string
	version  []int
}

// Compare returns 1 if this Version comes before the other, -1 if this Version
// comes after the other, and 0 if they are equal.
func (v *Version) Compare(other *Version) int {
	// Compare the elements of each slice, in order.
	for i := 0; i < len(v.version) && i < len(other.version); i++ {
		if v.version[i] < other.version[i] {
			return 1
		} else if v.version[i] > other.version[i] {
			return -1
		}
	}
	// If the slices are the same length and all elements were equal, the
	// versions are equal.
	if len(v.version) == len(other.version) {
		return 0
	}
	// If the slices are different lengths, the shorter one sorts before the
	// longer one in increasing order, eg. "1.0" vs "1.0.1".
	if len(v.version) < len(other.version) {
		return 1
	} else {
		return -1
	}
}

// String returns the full version string.
func (v *Version) String() string {
	return v.asString
}

// parseVersion parses a sequence of integers from the given version.
func parseVersion(regex *regexp.Regexp, version string) ([]int, string, error) {
	matches := regex.FindStringSubmatch(version)
	if len(matches) > 1 {
		version = matches[0]
		matchInts := make([]int, 0, len(matches)-1)
		for idx, a := range matches[1:] {
			i, err := strconv.Atoi(a)
			if err != nil && idx == 0 {
				// Allow the first capture group to be a substring used for the
				// version
				version = a
			} else if err != nil {
				return nil, "", skerr.Wrapf(err, "failed to parse int from regex match string; is the regex incorrect?")
			} else {
				matchInts = append(matchInts, i)
			}
		}
		return matchInts, version, nil
	}
	return nil, "", ErrNoMatch
}

// VersionSlice implements sort.Interface.
type VersionSlice []*Version

// Len implements sort.Interface.
func (s VersionSlice) Len() int {
	return len(s)
}

// Less implements sort.Interface.
func (s VersionSlice) Less(i, j int) bool {
	return s[i].Compare(s[j]) >= 1
}

// Swap implements sort.Interface.
func (s VersionSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Parser is used for parsing Versions.
type Parser struct {
	regex *regexp.Regexp
}

// NewParser returns a Parser instance.
//
// The Regexp should contain a series of groups matching only integers. These
// are extracted and compared in order of appearance when comparing Versions.
func NewParser(regex string) (*Parser, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse regular expression for semantic version parser")
	}
	return &Parser{
		regex: re,
	}, nil
}

// Parse and return a new semantic Version.
func (p *Parser) Parse(version string) (*Version, error) {
	ints, parsedVersion, err := parseVersion(p.regex, version)
	if err != nil {
		return nil, err
	}
	return &Version{
		asString: parsedVersion,
		version:  ints,
	}, nil
}

// Regex returns the wrapped regexp.Regexp.
func (p *Parser) Regex() *regexp.Regexp {
	return p.regex
}
