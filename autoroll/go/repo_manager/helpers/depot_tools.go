package helpers

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

var getDepRegex = regexp.MustCompile("[a-f0-9]+")

// GetDEPSFile downloads and returns the path to the DEPS file, and a cleanup
// function to run when finished with it.
func GetDEPSFile(ctx context.Context, repo *gitiles.Repo, gclient, baseCommit string) (rv string, cleanup func(), rvErr error) {
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if rvErr != nil {
			util.RemoveAll(wd)
		}
	}()

	// Download the DEPS file from the parent repo.
	buf := bytes.NewBuffer([]byte{})
	if err := repo.ReadFileAtRef(ctx, "DEPS", baseCommit, buf); err != nil {
		return "", nil, err
	}

	// Use "gclient getdep" to retrieve the last roll revision.

	// "gclient getdep" requires a .gclient file.
	if _, err := exec.RunCwd(ctx, wd, "python", gclient, "config", repo.URL); err != nil {
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
	return depsFile, func() { util.RemoveAll(wd) }, nil
}

func GetDep(ctx context.Context, repo *gitiles.Repo, gclient, depsFile, depPath string) (string, error) {
	output, err := exec.RunCwd(ctx, path.Dir(depsFile), "python", gclient, "getdep", "-r", depPath)
	if err != nil {
		return "", err
	}
	splitGetdep := strings.Split(strings.TrimSpace(output), "\n")
	rev := strings.TrimSpace(splitGetdep[len(splitGetdep)-1])
	if getDepRegex.MatchString(rev) {
		if len(rev) == 40 {
			return rev, nil
		}
		// The DEPS entry may be a shortened commit hash. Try to resolve
		// the full hash.
		rev, err = repo.ResolveRef(ctx, rev)
		if err != nil {
			return "", skerr.Wrapf(err, "`gclient getdep` produced what appears to be a shortened commit hash, but failed to resolve it as a commit via gitiles. Output of `gclient getdep`:\n%s", output)
		}
		return rev, nil
	}
	return "", fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
}

func SetDep(ctx context.Context, gclient, depsFile, depPath, rev string) error {
	args := []string{"setdep", "-r", fmt.Sprintf("%s@%s", depPath, rev)}
	_, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  path.Dir(depsFile),
		Env:  depot_tools.Env(filepath.Dir(gclient)),
		Name: gclient,
		Args: args,
	})
	return err
}
