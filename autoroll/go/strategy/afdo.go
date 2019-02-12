package strategy

/*
   NextRollStrategy for AFDO versions.
*/

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	AFDO_GS_BUCKET = "chromeos-prebuilt"
	AFDO_GS_PATH   = "afdo-job/llvm/"

	AFDO_VERSION_LENGTH               = 5
	AFDO_VERSION_REGEX_EXPECT_MATCHES = AFDO_VERSION_LENGTH + 1
)

var (
	// Example name: chromeos-chrome-amd64-63.0.3239.57_rc-r1.afdo.bz2
	AFDO_VERSION_REGEX = regexp.MustCompile(
		"^chromeos-chrome-amd64-" + // Prefix
			"(\\d+)\\.(\\d+)\\.(\\d+)\\.(\\d+)" + // Version
			"_rc-r(\\d+)" + // Revision
			"-merged\\.afdo\\.bz2$") // Suffix

	// Error used to indicate that a version number is invalid.
	errInvalidAFDOVersion = errors.New("Invalid AFDO version.")
)

// Parse the AFDO version.
func parseAFDOVersion(ver string) ([AFDO_VERSION_LENGTH]int, error) {
	matches := AFDO_VERSION_REGEX.FindStringSubmatch(ver)
	var matchInts [AFDO_VERSION_LENGTH]int
	if len(matches) == AFDO_VERSION_REGEX_EXPECT_MATCHES {
		for idx, a := range matches[1:] {
			i, err := strconv.Atoi(a)
			if err != nil {
				return matchInts, fmt.Errorf("Failed to parse int from regex match string; is the regex incorrect?")
			}
			matchInts[idx] = i
		}
		return matchInts, nil
	} else {
		return matchInts, errInvalidAFDOVersion
	}
}

// Return true iff version a is greater than version b.
func AFDOVersionGreater(a, b string) (bool, error) {
	verA, err := parseAFDOVersion(a)
	if err != nil {
		return false, err
	}
	verB, err := parseAFDOVersion(b)
	if err != nil {
		return false, err
	}
	for i := 0; i < AFDO_VERSION_LENGTH; i++ {
		if verA[i] > verB[i] {
			return true, nil
		} else if verA[i] < verB[i] {
			return false, nil
		}
	}
	return false, nil
}

type afdoVersionSlice []string

func (s afdoVersionSlice) Len() int {
	return len(s)
}

func (s afdoVersionSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// We sort newest to oldest.
func (s afdoVersionSlice) Less(i, j int) bool {
	greater, err := AFDOVersionGreater(s[i], s[j])
	if err != nil {
		// We should've caught any parsing errors before we inserted the
		// versions into the slice.
		sklog.Errorf("Failed to compare AFDO versions: %s", err)
	}
	return greater
}

// AFDOStrategy is a NextRollStrategy which chooses the most recent AFDO profile
// to roll.
type AFDOStrategy struct {
	gcs      gcs.GCSClient
	mtx      sync.Mutex
	versions []string
}

// See documentation for Strategy interface.
func (s *AFDOStrategy) GetNextRollRev(ctx context.Context, _ []*vcsinfo.LongCommit) (string, error) {
	// Find the available AFDO versions, sorted newest to oldest, and store.
	available := []string{}
	if err := s.gcs.AllFilesInDirectory(ctx, AFDO_GS_PATH, func(item *storage.ObjectAttrs) {
		name := strings.TrimPrefix(item.Name, AFDO_GS_PATH)
		if _, err := parseAFDOVersion(name); err == nil {
			available = append(available, name)
		} else if err == errInvalidAFDOVersion {
			// There are files we don't care about in this bucket. Just ignore.
		} else {
			sklog.Error(err)
		}
	}); err != nil {
		return "", err
	}
	if len(available) == 0 {
		return "", fmt.Errorf("No valid AFDO profile names found.")
	}
	sort.Sort(afdoVersionSlice(available))

	// Store the available versions. Return the newest.
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.versions = available
	return s.versions[0], nil
}

// Return the list of versions.
func (s *AFDOStrategy) GetVersions() []string {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.versions
}
