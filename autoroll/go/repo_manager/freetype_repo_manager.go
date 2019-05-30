package repo_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/util"
)

const (
	ftReadmePath         = "third_party/freetype/README.chromium"
	ftReadmeVersionTmpl  = "%sVersion: %s"
	ftReadmeRevisionTmpl = "%sRevision: %s"

	ftIncludeSrc  = "include/freetype/config"
	ftIncludeDest = "third_party/freetype/include/freetype-custom-config"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewFreeTypeRepoManager func(context.Context, *FreeTypeRepoManagerConfig, string, gerrit.GerritInterface, string, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newFreeTypeRepoManager

	ftReadmeVersionRegex  = regexp.MustCompile(fmt.Sprintf(ftReadmeVersionTmpl, "(?m)^", ".*"))
	ftReadmeRevisionRegex = regexp.MustCompile(fmt.Sprintf(ftReadmeRevisionTmpl, "(?m)^", ".*"))

	ftIncludesToMerge = []string{
		"ftoption.h",
		"ftconfig.h",
	}
)

// FreeTypeRepoManagerConfig provides configuration for FreeTypeRepoManager.
type FreeTypeRepoManagerConfig struct {
	NoCheckoutDEPSRepoManagerConfig
}

// freeTypeRepoManager is a RepoManager which rolls FreeType in DEPS and updates
// header files and README.chromium.
type freetypeRepoManager struct {
	*noCheckoutDEPSRepoManager
	localChildRepo *git.Repo
}

// newFreeTypeRepoManager returns a RepoManager instance which rolls FreeType
// in DEPS and updates header files and README.chromium.
func newFreeTypeRepoManager(ctx context.Context, c *FreeTypeRepoManagerConfig, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL, gitcookiesPath string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	ncrm, err := newNoCheckoutDEPSRepoManager(ctx, &c.NoCheckoutDEPSRepoManagerConfig, workdir, g, recipeCfgFile, serverURL, gitcookiesPath, client, cr, local)
	if err != nil {
		return nil, err
	}
	localChildRepo, err := git.NewRepo(ctx, c.ChildRepo, workdir)
	if err != nil {
		return nil, err
	}
	rv := &freetypeRepoManager{
		localChildRepo:            localChildRepo,
		noCheckoutDEPSRepoManager: ncrm.(*noCheckoutDEPSRepoManager),
	}
	rv.noCheckoutDEPSRepoManager.noCheckoutRepoManager.createRoll = rv.createRoll
	rv.noCheckoutDEPSRepoManager.noCheckoutRepoManager.updateHelper = rv.updateHelper
	return rv, nil
}

// Perform a three-way merge for this header file in a temporary dir. Adds the
// new contents to the changes map.
func (rm *freetypeRepoManager) mergeInclude(ctx context.Context, include, from, to string, changes map[string]string) error {
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer util.RemoveAll(wd)

	gd := git.GitDir(wd)
	_, err = gd.Git(ctx, "init")

	// Obtain the current version of the file in the parent repo.
	parentHeader := path.Join(ftIncludeDest, include)
	dest := filepath.Join(wd, include)
	var buf bytes.Buffer
	if err := rm.parentRepo.ReadFileAtRef(parentHeader, rm.baseCommit, &buf); err != nil {
		return err
	}
	oldParentContents := buf.String()
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(dest, buf.Bytes(), os.ModePerm); err != nil {
		return err
	}
	if _, err := gd.Git(ctx, "add", dest); err != nil {
		return err
	}
	if _, err := gd.Git(ctx, "commit", "-m", "fake"); err != nil {
		return err
	}

	// Obtain the old version of the file in the child repo.
	ftHeader := path.Join(ftIncludeSrc, include)
	oldChildContents, err := rm.localChildRepo.GetFile(ctx, ftHeader, from)
	if err != nil {
		return err
	}
	oldPath := filepath.Join(wd, "old")
	if err := ioutil.WriteFile(oldPath, []byte(oldChildContents), os.ModePerm); err != nil {
		return err
	}

	// Obtain the new version of the file in the child repo.
	newChildContents, err := rm.localChildRepo.GetFile(ctx, ftHeader, to)
	if err != nil {
		return err
	}
	newPath := filepath.Join(wd, "new")
	if err := ioutil.WriteFile(newPath, []byte(newChildContents), os.ModePerm); err != nil {
		return err
	}

	// Perform the merge.
	if _, err := gd.Git(ctx, "merge-file", dest, oldPath, newPath); err != nil {
		return err
	}

	// Read the resulting contents.
	newParentContents, err := ioutil.ReadFile(dest)
	if err != nil {
		return err
	}
	if string(newParentContents) != string(oldParentContents) {
		changes[parentHeader] = string(newParentContents)
	}
	return nil
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *freetypeRepoManager) createRoll(ctx context.Context, from, to, serverURL, cqExtraTrybots string, emails []string) (string, map[string]string, error) {
	commitMsg, changes, err := rm.noCheckoutDEPSRepoManager.createRoll(ctx, from, to, serverURL, cqExtraTrybots, emails)
	if err != nil {
		return "", nil, err
	}

	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()

	// Update README.chromium.
	ftVersion, err := rm.localChildRepo.Git(ctx, "describe", "--long", to)
	if err != nil {
		return "", nil, err
	}
	ftVersion = strings.TrimSpace(ftVersion)
	var buf bytes.Buffer
	if err := rm.parentRepo.ReadFileAtRef(ftReadmePath, rm.baseCommit, &buf); err != nil {
		return "", nil, err
	}
	oldReadmeContents := buf.String()
	newReadmeContents := ftReadmeVersionRegex.ReplaceAllString(oldReadmeContents, fmt.Sprintf(ftReadmeVersionTmpl, "", ftVersion))
	newReadmeContents = ftReadmeRevisionRegex.ReplaceAllString(newReadmeContents, fmt.Sprintf(ftReadmeRevisionTmpl, "", to))
	if newReadmeContents != oldReadmeContents {
		changes[ftReadmePath] = newReadmeContents
	}

	// Merge includes.
	for _, include := range ftIncludesToMerge {
		if err := rm.mergeInclude(ctx, include, from, to, changes); err != nil {
			return "", nil, err
		}
	}

	// Check modules.cfg. Give up if it has changed.
	diff, err := rm.localChildRepo.Git(ctx, "diff", "--name-only", fmt.Sprintf("%s..%s", from, to))
	if err != nil {
		return "", nil, err
	}
	if strings.Contains(diff, "modules.cfg") {
		return "", nil, errors.New("modules.cfg has been modified; cannot roll automatically.")
	}

	return commitMsg, changes, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *freetypeRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (string, string, []*revision.Revision, error) {
	lastRollRev, nextRollRev, notRolledRevs, err := rm.noCheckoutDEPSRepoManager.updateHelper(ctx, strat, parentRepo, baseCommit)
	if err != nil {
		return "", "", nil, err
	}
	if err := rm.localChildRepo.Update(ctx); err != nil {
		return "", "", nil, err
	}
	return lastRollRev, nextRollRev, notRolledRevs, nil
}
