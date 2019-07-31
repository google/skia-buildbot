package repo_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

/*
	Repo manager which rolls Android AFDO profiles into Chromium.
*/

const (
	TMPL_COMMIT_MSG_AFDO = `Roll AFDO from {{.RollingFrom.String}} to {{.RollingTo.String}}

This CL may cause a small binary size increase, roughly proportional
to how long it's been since our last AFDO profile roll. For larger
increases (around or exceeding 100KB), please file a bug against
gbiv@chromium.org. Additional context: https://crbug.com/805539

Please note that, despite rolling to chrome/android, this profile is
used for both Linux and Android.

The AutoRoll server is located here: {{.ServerURL}}

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

If the roll is causing failures, please contact the current sheriff, who should
be CC'd on the roll, and stop the roller if necessary.

{{if .CqExtraTrybots}}CQ_INCLUDE_TRYBOTS={{.CqExtraTrybots}}
{{end}}TBR={{stringsJoin .Reviewers ","}}
`

	AFDO_GS_BUCKET = "chromeos-prebuilt"
	AFDO_GS_PATH   = "afdo-job/llvm/"

	AFDO_VERSION_LENGTH               = 4
	AFDO_VERSION_REGEX_EXPECT_MATCHES = AFDO_VERSION_LENGTH + 1
)

var (
	// "Constants"

	AFDO_VERSION_FILE_PATH = path.Join("chrome", "android", "profiles", "newest.txt")

	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewAFDORepoManager func(context.Context, *AFDORepoManagerConfig, string, gerrit.GerritInterface, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newAfdoRepoManager

	// Example name: chromeos-chrome-amd64-63.0.3239.57_rc-r1.afdo.bz2
	AFDO_VERSION_REGEX = regexp.MustCompile(
		"^chromeos-chrome-amd64-" + // Prefix
			"(\\d+)\\.(\\d+)\\.(\\d+)\\.0" + // Version
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

// Shorten the AFDO version.
func afdoShortVersion(long string) string {
	return strings.TrimPrefix(strings.TrimSuffix(long, ".afdo.bz2"), "chromeos-chrome-amd64-")
}

// AFDORepoManagerConfig provides configuration for the AFDO RepoManager.
type AFDORepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
}

// afdoRepoManager is a RepoManager which rolls Android AFDO profile version
// numbers into Chromium. Unlike other rollers, there is no child repo to sync;
// the version number is obtained from Google Cloud Storage.
type afdoRepoManager struct {
	*noCheckoutRepoManager
	afdoVersionFile string
	authClient      *http.Client
	gcs             gcs.GCSClient
	versions        []string // Protected by infoMtx.
}

// Return an afdoRepoManager instance.
func newAfdoRepoManager(ctx context.Context, c *AFDORepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	gcsClient := gcsclient.New(storageClient, AFDO_GS_BUCKET)
	rv := &afdoRepoManager{
		afdoVersionFile: AFDO_VERSION_FILE_PATH,
		authClient:      client,
		gcs:             gcsClient,
	}
	if c.CommitMsgTmpl == "" {
		c.CommitMsgTmpl = TMPL_COMMIT_MSG_AFDO
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, workdir, g, serverURL, gitcookiesPath, client, cr, rv.createRoll, rv.updateHelper, local)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *afdoRepoManager) createRoll(ctx context.Context, from, to *revision.Revision, serverURL, cqExtraTrybots string, emails []string) (string, map[string]string, error) {
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
	return commitMsg, map[string]string{AFDO_VERSION_FILE_PATH: to.Id}, nil
}

func afdoVersionToRevision(ver string) *revision.Revision {
	return &revision.Revision{
		Id:      ver,
		Display: afdoShortVersion(ver),
	}
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *afdoRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Read the version file to determine the last roll rev.
	buf := bytes.NewBuffer([]byte{})
	if err := parentRepo.ReadFileAtRef(rm.afdoVersionFile, baseCommit, buf); err != nil {
		return nil, nil, nil, err
	}
	lastRollRev := afdoVersionToRevision(strings.TrimSpace(buf.String()))

	// Find the available AFDO versions, sorted newest to oldest.
	versions := []string{}
	if err := rm.gcs.AllFilesInDirectory(ctx, AFDO_GS_PATH, func(item *storage.ObjectAttrs) {
		name := strings.TrimPrefix(item.Name, AFDO_GS_PATH)
		if _, err := parseAFDOVersion(name); err == nil {
			versions = append(versions, name)
		} else if err == errInvalidAFDOVersion {
			// There are files we don't care about in this bucket. Just ignore.
		} else {
			sklog.Error(err)
		}
	}); err != nil {
		return nil, nil, nil, err
	}
	if len(versions) == 0 {
		return nil, nil, nil, fmt.Errorf("No valid AFDO profile names found.")
	}
	sort.Sort(afdoVersionSlice(versions))

	lastIdx := -1
	for idx, v := range versions {
		if v == lastRollRev.Id {
			lastIdx = idx
			break
		}
	}
	if lastIdx == -1 {
		return nil, nil, nil, fmt.Errorf("Last roll rev %q not found in available versions. Unable to create revision list.", lastRollRev)
	}

	// Get the list of not-yet-rolled revisions.
	notRolledRevs := make([]*revision.Revision, 0, len(versions)-lastIdx)
	for i := 0; i < lastIdx; i++ {
		notRolledRevs = append(notRolledRevs, afdoVersionToRevision(versions[i]))
	}
	nextRollRev, err := rm.getNextRollRev(ctx, notRolledRevs, lastRollRev)
	if err != nil {
		return nil, nil, nil, err
	}
	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.versions = versions
	return lastRollRev, nextRollRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) RolledPast(ctx context.Context, rev *revision.Revision) (bool, error) {
	verIsNewer, err := AFDOVersionGreater(rev.Id, rm.LastRollRev().Id)
	if err != nil {
		return false, err
	}
	return !verIsNewer, nil
}

// See documentation for RepoManager interface.
func (r *afdoRepoManager) ValidStrategies() []string {
	return []string{strategy.ROLL_STRATEGY_BATCH}
}

// See documentation for RepoManager interface.
func (r *afdoRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	return afdoVersionToRevision(id), nil
}
