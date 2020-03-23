package parent

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

type GitilesDEPSConfig struct {
	GitilesConfig
	DepPath string `json:"depPath"`
	DepURL  string `json:"depURL"`
}

// TODO: validate

// NewGitilesDEPS returns a Parent implementation which uses Gitiles to roll
// DEPS.
func NewGitilesDEPS(ctx context.Context, c GitilesDEPSConfig, client *http.Client, serverURL string) (Parent, error) {
	depotTools := "TODO"

	update := func(ctx context.Context, repo *gitiles.Repo, baseCommit string) (string, error) {
		// Download the DEPS file from the parent repo.
		depsFile, cleanup, err := GetDEPSFile(ctx, repo, depotTools, baseCommit)
		if err != nil {
			return "", skerr.Wrapf(err, "Failed to retrieve DEPS file")
		}
		defer cleanup()

		// Read the last-rolled rev from the DEPS file.
		return depot_tools.GetDEP(ctx, depotTools, depsFile, c.DepPath)
	}

	getChangesForRoll := func(ctx context.Context, repo *gitiles.Repo, baseCommit string, from, to *revision.Revision, rolling []*revision.Revision) (map[string]string, error) {
		// Download the DEPS file from the parent repo.
		depsFile, cleanup, err := GetDEPSFile(ctx, repo, depotTools, baseCommit)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to retrieve DEPS file")
		}
		defer cleanup()

		// Write the new DEPS content.
		if err := depot_tools.SetDep(ctx, depotTools, depsFile, c.DepPath, to.Id); err != nil {
			return nil, skerr.Wrapf(err, "Failed to set new revision")
		}

		// TODO(borenet): Support transitive DEPS!

		// Read the updated DEPS content.
		newDEPSContent, err := ioutil.ReadFile(depsFile)
		if err != nil {
			return nil, err
		}
		return map[string]string{"DEPS": string(newDEPSContent)}, nil
	}
	return newGitiles(ctx, c.GitilesConfig, client, serverURL, update, getChangesForRoll)
}

// GetDEPSFile downloads and returns the path to the DEPS file, and a cleanup
// function to run when finished with it.
func GetDEPSFile(ctx context.Context, repo *gitiles.Repo, depotTools, baseCommit string) (rv string, cleanup func(), rvErr error) {
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() {
		util.RemoveAll(wd)
	}
	defer func() {
		if rvErr != nil {
			cleanup()
		}
	}()

	// Download the DEPS file from the parent repo.
	buf := bytes.NewBuffer([]byte{})
	if err := repo.ReadFileAtRef(ctx, "DEPS", baseCommit, buf); err != nil {
		return "", nil, err
	}

	// "gclient getdep" requires a .gclient file.
	if _, err := depot_tools.RunWithEnv(ctx, depotTools, wd, "python", filepath.Join(depotTools, "gclient.py"), "config", repo.URL); err != nil {
		return "", nil, err
	}
	splitRepo := strings.Split(repo.URL, "/")
	fakeCheckoutDir := path.Join(wd, strings.TrimSuffix(splitRepo[len(splitRepo)-1], ".git"))
	if err := os.Mkdir(fakeCheckoutDir, os.ModePerm); err != nil {
		return "", nil, err
	}
	depsFile := path.Join(fakeCheckoutDir, "DEPS")
	if err := ioutil.WriteFile(depsFile, buf.Bytes(), os.ModePerm); err != nil {
		return "", nil, err
	}
	return depsFile, cleanup, nil
}
