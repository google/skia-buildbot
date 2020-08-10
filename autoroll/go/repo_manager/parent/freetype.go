package parent

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
)

const (
	FtReadmePath         = "third_party/freetype/README.chromium"
	FtReadmeVersionTmpl  = "%sVersion: %s"
	FtReadmeRevisionTmpl = "%sRevision: %s"

	FtIncludeSrc  = "include"
	FtIncludeDest = "third_party/freetype/include/freetype-custom"
)

var (
	FtReadmeVersionRegex  = regexp.MustCompile(fmt.Sprintf(FtReadmeVersionTmpl, "(?m)^", ".*"))
	FtReadmeRevisionRegex = regexp.MustCompile(fmt.Sprintf(FtReadmeRevisionTmpl, "(?m)^", ".*"))

	FtIncludesToMerge = []string{
		"freetype/config/ftoption.h",
		"freetype/config/public-macros.h",
	}
)

func NewFreeTypeParent(ctx context.Context, c GitilesConfig, reg *config_vars.Registry, workdir string, client *http.Client, serverURL string) (*gitilesParent, error) {
	localChildRepo, err := git.NewRepo(ctx, c.DependencyConfig.ID, workdir)
	if err != nil {
		return nil, err
	}
	getChangesHelper := gitilesFileGetChangesForRollFunc(c.DependencyConfig)
	getChangesForRoll := func(ctx context.Context, parentRepo *gitiles_common.GitilesRepo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, error) {
		// Get the DEPS changes via gitilesDEPSGetChangesForRollFunc.
		changes, err := getChangesHelper(ctx, parentRepo, baseCommit, from, to, rolling)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		// Update README.chromium.
		if err := localChildRepo.Update(ctx); err != nil {
			return nil, skerr.Wrap(err)
		}
		ftVersion, err := localChildRepo.Git(ctx, "describe", "--long", to.Id)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		ftVersion = strings.TrimSpace(ftVersion)
		fs, err := parentRepo.VFS(ctx, &revision.Revision{Id: baseCommit})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		oldReadmeBytes, err := vfs.ReadFile(ctx, fs, FtReadmePath)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		oldReadmeContents := string(oldReadmeBytes)
		newReadmeContents := FtReadmeVersionRegex.ReplaceAllString(oldReadmeContents, fmt.Sprintf(FtReadmeVersionTmpl, "", ftVersion))
		newReadmeContents = FtReadmeRevisionRegex.ReplaceAllString(newReadmeContents, fmt.Sprintf(FtReadmeRevisionTmpl, "", to.Id))
		if newReadmeContents != oldReadmeContents {
			changes[FtReadmePath] = newReadmeContents
		}

		// Merge includes.
		for _, include := range FtIncludesToMerge {
			if err := mergeInclude(ctx, include, from.Id, to.Id, fs, changes, parentRepo, localChildRepo); err != nil {
				return nil, skerr.Wrap(err)
			}
		}

		// Check modules.cfg. Give up if it has changed.
		diff, err := localChildRepo.Git(ctx, "diff", "--name-only", git.LogFromTo(from.Id, to.Id))
		if err != nil {
			return nil, err
		}
		if strings.Contains(diff, "modules.cfg") {
			return nil, skerr.Fmt("modules.cfg has been modified; cannot roll automatically.")
		}
		return changes, nil
	}
	return newGitiles(ctx, c, reg, client, serverURL, getChangesForRoll)
}

// Perform a three-way merge for this header file in a temporary dir. Adds the
// new contents to the changes map.
func mergeInclude(ctx context.Context, include, from, to string, fs vfs.FS, changes map[string]string, parentRepo *gitiles_common.GitilesRepo, localChildRepo *git.Repo) error {
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.RemoveAll(wd)

	gd := git.GitDir(wd)
	_, err = gd.Git(ctx, "init")
	if err != nil {
		return skerr.Wrap(err)
	}

	// Obtain the current version of the file in the parent repo.
	parentHeader := path.Join(FtIncludeDest, include)
	dest := filepath.Join(wd, include)
	oldParentBytes, err := vfs.ReadFile(ctx, fs, parentHeader)
	if err != nil {
		return skerr.Wrap(err)
	}
	oldParentContents := string(oldParentBytes)
	if err != nil {
		return skerr.Wrap(err)
	}
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}
	if err := ioutil.WriteFile(dest, oldParentBytes, os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := gd.Git(ctx, "add", dest); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := gd.Git(ctx, "commit", "-m", "fake"); err != nil {
		return skerr.Wrap(err)
	}

	// Obtain the old version of the file in the child repo.
	ftHeader := path.Join(FtIncludeSrc, include)
	oldChildContents, err := localChildRepo.GetFile(ctx, ftHeader, from)
	if err != nil {
		return skerr.Wrap(err)
	}
	oldPath := filepath.Join(wd, "old")
	if err := ioutil.WriteFile(oldPath, []byte(oldChildContents), os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}

	// Obtain the new version of the file in the child repo.
	newChildContents, err := localChildRepo.GetFile(ctx, ftHeader, to)
	if err != nil {
		return skerr.Wrap(err)
	}
	newPath := filepath.Join(wd, "new")
	if err := ioutil.WriteFile(newPath, []byte(newChildContents), os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}

	// Perform the merge.
	if _, err := gd.Git(ctx, "merge-file", dest, oldPath, newPath); err != nil {
		return skerr.Wrap(err)
	}

	// Read the resulting contents.
	newParentContents, err := ioutil.ReadFile(dest)
	if err != nil {
		return skerr.Wrap(err)
	}
	if string(newParentContents) != string(oldParentContents) {
		changes[parentHeader] = string(newParentContents)
	}
	return nil
}
