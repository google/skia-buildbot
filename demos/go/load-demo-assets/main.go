package main

// Loads all checked in demos from infra-internal to be bundled into the demoserver image for
// release.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"

	"go.skia.org/infra/go/sklog"
)

var (
	demosRepo     = flag.String("repo_url", "https://skia.googlesource.com/infra-internal", "The repo from where to fetch the demos. Defaults to https://skia.googlesource.com/infra-internal")
	demosRepoPath = flag.String("demos_dir", "scripts", "The top level directory in the repo that holds the demos.")
	outDir        = flag.String("out_dir", "./out", "Where the demos from demos_dir should be downloaded to, directories will be created as needed.")
)

func getMetadata(repo *gitiles.Repo, dir string) (*vcsinfo.LongCommit, error) {
	log, err := repo.Log(context.Background(), fmt.Sprintf("master/%s", dir), gitiles.LogLimit(1))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed loading %s", dir)
	}
	if len(log) != 1 {
		return nil, skerr.Fmt("Failed to obtain the last commit which modified %s in %s; expected 1 commit but got %d", dir, repo.URL, len(log))
	}
	return log[0], nil
}

func main() {
	flag.Parse()
	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	repo := gitiles.NewRepo(*demosRepo, client)
	files, err := repo.ListFilesRecursive(context.Background(), *demosRepoPath)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Downloading files: %v", files)
	demoDirsMetadata := make(map[string]*vcsinfo.LongCommit)
	for _, f := range files {
		fullFilepath := filepath.Join(*demosRepoPath, f)
		err := repo.DownloadFile(context.Background(), fullFilepath, filepath.Join(*outDir, f))
		if err != nil {
			sklog.Fatal(err)
		}
		demoName := filepath.Dir(f)
		if demoDirsMetadata[demoName] == nil {
			// When we see a new directory, fetch author/blame, etc.
			demoDirsMetadata[demoName], err = getMetadata(repo, filepath.Dir(fullFilepath))
			if err != nil {
				sklog.Fatal(err)
			}
		}
	}
	obj, err := json.MarshalIndent(demoDirsMetadata, "", "  ")
	if err != nil {
		sklog.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(*outDir, "metadata.json"), obj, 0644)
	if err != nil {
		sklog.Fatal(err)
	}

}
