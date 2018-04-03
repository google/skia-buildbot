package repo_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

/*
	Repo manager which rolls Android AFDO profiles into Chromium.
*/

const (
	AFDO_GS_BUCKET = "chromeos-prebuilt"
	AFDO_GS_PATH   = "afdo-job/llvm/"

	AFDO_VERSION_LENGTH               = 5
	AFDO_VERSION_REGEX_EXPECT_MATCHES = AFDO_VERSION_LENGTH + 1

	AFDO_COMMIT_MSG_TMPL = `Roll AFDO from %s to %s

This CL may cause a small binary size increase, roughly proportional
to how long it's been since our last AFDO profile roll. For larger
increases (around or exceeding 100KB), please file a bug against
gbiv@chromium.org. Additional context: https://crbug.com/805539
` + COMMIT_MSG_FOOTER_TMPL
)

var (
	// "Constants"

	// Example name: chromeos-chrome-amd64-63.0.3239.57_rc-r1.afdo.bz2
	AFDO_VERSION_REGEX = regexp.MustCompile(
		"^chromeos-chrome-amd64-" + // Prefix
			"(\\d+)\\.(\\d+)\\.(\\d+)\\.(\\d+)" + // Version
			"_rc-r(\\d+)" + // Revision
			"\\.afdo\\.bz2$") // Suffix

	AFDO_VERSION_FILE_PATH = path.Join("chrome", "android", "profiles", "newest.txt")

	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewAFDORepoManager func(context.Context, *AFDORepoManagerConfig, string, *gerrit.Gerrit, string, string, *http.Client) (RepoManager, error) = newAfdoRepoManager

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
func afdoVersionGreater(a, b string) (bool, error) {
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
	greater, err := afdoVersionGreater(s[i], s[j])
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

// afdoStrategy is a NextRollStrategy which chooses the most recent AFDO profile
// to roll.
type afdoStrategy struct {
	gcs      gcs.GCSClient
	mtx      sync.Mutex
	versions []string
}

// See documentation for Strategy interface.
func (s *afdoStrategy) GetNextRollRev(ctx context.Context, _ *git.Checkout, _ string) (string, error) {
	// Find the available AFDO versions, sorted newest to oldest, and store.
	available := []string{}
	if err := s.gcs.AllFilesInDirectory(ctx, AFDO_GS_PATH, func(item *storage.ObjectAttrs) {
		name := strings.TrimPrefix(item.Name, AFDO_GS_PATH)
		if _, err := parseAFDOVersion(name); err == nil {
			available = append(available, name)
		} else if err == errInvalidAFDOVersion {
			sklog.Warningf("Found AFDO file with improperly formatted name: %s", name)
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
func (s *afdoStrategy) GetVersions() []string {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.versions
}

// AFDORepoManagerConfig provides configuration for the AFDO RepoManager.
type AFDORepoManagerConfig struct {
	DepotToolsRepoManagerConfig
}

// Validate the config.
func (c *AFDORepoManagerConfig) Validate() error {
	if c.Strategy != ROLL_STRATEGY_AFDO {
		return errors.New("No custom strategy allowed for AFDO RepoManager.")
	}
	return c.DepotToolsRepoManagerConfig.Validate()
}

// afdoRepoManager is a RepoManager which rolls Android AFDO profile version
// numbers into Chromium. Unlike other rollers, there is no child repo to sync;
// the version number is obtained from Google Cloud Storage.
type afdoRepoManager struct {
	*depotToolsRepoManager
	afdoVersionFile  string
	commitsNotRolled int      // Protected by infoMtx.
	versions         []string // Protected by infoMtx.
}

func newAfdoRepoManager(ctx context.Context, c *AFDORepoManagerConfig, workdir string, g *gerrit.Gerrit, recipeCfgFile, serverURL string, authClient *http.Client) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	drm, err := newDepotToolsRepoManager(ctx, c.DepotToolsRepoManagerConfig, path.Join(workdir, "repo_manager"), recipeCfgFile, serverURL, g)
	if err != nil {
		return nil, err
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(authClient))
	if err != nil {
		return nil, err
	}
	drm.strategy = &afdoStrategy{
		gcs: gcs.NewGCSClient(storageClient, AFDO_GS_BUCKET),
	}

	rv := &afdoRepoManager{
		afdoVersionFile:       path.Join(drm.parentDir, AFDO_VERSION_FILE_PATH),
		depotToolsRepoManager: drm,
	}
	return rv, rv.Update(ctx)
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	// Clean the checkout, get onto a fresh branch.
	if err := rm.cleanParent(ctx); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "checkout", "-b", ROLL_BRANCH, "-t", fmt.Sprintf("origin/%s", rm.parentBranch), "-f"); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(rm.cleanParent(ctx))
	}()

	// Create the roll CL.
	if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "config", "user.name", rm.user); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, rm.parentDir, "git", "config", "user.email", rm.user); err != nil {
		return 0, err
	}

	// Write the file.
	if err := ioutil.WriteFile(rm.afdoVersionFile, []byte(to), os.ModePerm); err != nil {
		return 0, err
	}

	// Commit.
	commitMsg := fmt.Sprintf(AFDO_COMMIT_MSG_TMPL, afdoShortVersion(from), afdoShortVersion(to), rm.serverURL)
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.parentDir,
		Env:  rm.GetEnvForDepotTools(),
		Name: "git",
		Args: []string{"commit", "-a", "-m", commitMsg},
	}); err != nil {
		return 0, err
	}

	// Upload the CL.
	uploadCmd := &exec.Command{
		Dir:     rm.parentDir,
		Env:     rm.GetEnvForDepotTools(),
		Name:    "git",
		Args:    []string{"cl", "upload", "--bypass-hooks", "-f", "-v", "-v"},
		Timeout: 2 * time.Minute,
	}
	if dryRun {
		uploadCmd.Args = append(uploadCmd.Args, "--cq-dry-run")
	} else {
		uploadCmd.Args = append(uploadCmd.Args, "--use-commit-queue")
	}
	uploadCmd.Args = append(uploadCmd.Args, "--gerrit")
	tbr := "\nTBR="
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		tbr += emailStr
		uploadCmd.Args = append(uploadCmd.Args, "--send-mail", "--cc", emailStr)
	}
	commitMsg += tbr
	uploadCmd.Args = append(uploadCmd.Args, "-m", commitMsg)

	// Upload the CL.
	sklog.Infof("Running command: git %s", strings.Join(uploadCmd.Args, " "))
	if _, err := exec.RunCommand(ctx, uploadCmd); err != nil {
		return 0, err
	}

	// Obtain the issue number.
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return 0, err
	}
	defer util.RemoveAll(tmp)
	jsonFile := path.Join(tmp, "issue.json")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.parentDir,
		Env:  rm.GetEnvForDepotTools(),
		Name: "git",
		Args: []string{"cl", "issue", fmt.Sprintf("--json=%s", jsonFile)},
	}); err != nil {
		return 0, err
	}
	f, err := os.Open(jsonFile)
	if err != nil {
		return 0, err
	}
	var issue issueJson
	if err := json.NewDecoder(f).Decode(&issue); err != nil {
		return 0, err
	}
	return issue.Issue, nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) PreUploadSteps() []PreUploadStep {
	return nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) Update(ctx context.Context) error {
	// Sync the projects.
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	if err := rm.createAndSyncParent(ctx); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Read the file to determine the last roll rev.
	lastRollRevBytes, err := ioutil.ReadFile(rm.afdoVersionFile)
	if err != nil {
		return err
	}
	lastRollRev := strings.TrimSpace(string(lastRollRevBytes))

	// Get the next roll rev.
	nextRollRev, err := rm.strategy.GetNextRollRev(ctx, nil, "")
	if err != nil {
		return err
	}
	versions := rm.strategy.(*afdoStrategy).GetVersions()
	lastIdx := -1
	nextIdx := -1
	for idx, v := range versions {
		if v == lastRollRev {
			lastIdx = idx
		}
		if v == nextRollRev {
			nextIdx = idx
		}
	}
	if lastIdx == -1 {
		sklog.Errorf("Last roll rev %q not found in available versions. Not-rolled count will be wrong.", lastRollRev)
	}
	if nextIdx == -1 {
		sklog.Errorf("Next roll rev %q not found in available versions. Not-rolled count will be wrong.", nextRollRev)
	}

	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()
	rm.lastRollRev = lastRollRev
	rm.nextRollRev = nextRollRev
	// This seems backwards, but the versions are in descending order.
	rm.commitsNotRolled = lastIdx - nextIdx
	rm.versions = rm.strategy.(*afdoStrategy).GetVersions()
	return nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) FullChildHash(ctx context.Context, ver string) (string, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	for _, v := range rm.versions {
		if strings.Contains(v, ver) {
			return v, nil
		}
	}
	return "", fmt.Errorf("Unable to find version: %s", ver)
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) RolledPast(ctx context.Context, ver string) (bool, error) {
	verIsNewer, err := afdoVersionGreater(ver, rm.LastRollRev())
	if err != nil {
		return false, err
	}
	return !verIsNewer, nil
}

// See documentation for RepoManager interface.
func (rm *afdoRepoManager) CommitsNotRolled() int {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	return rm.commitsNotRolled
}
