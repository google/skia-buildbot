package main

// Loads all checked in demos from infra-internal to be bundled into the demoserver image for
// release.

import (
	"context"
	"flag"
	"path/filepath"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"

	"go.skia.org/infra/go/sklog"
)

var (
	demosRepo     = flag.String("repo_url", "https://skia.googlesource.com/infra-internal", "The repo from where to fetch the demos. Defaults to https://skia.googlesource.com/infra-internal")
	demosRepoPath = flag.String("demos_dir", "scripts", "The top level directory in the repo that holds the demos.")
	outDir        = flag.String("out_dir", "./out", "Where the demos from demos_dir should be downloaded to, directories will be created as needed.")
)

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

	for _, f := range files {
		err := repo.DownloadFile(context.Background(), filepath.Join(*demosRepoPath, f), filepath.Join(*outDir, f))
		if err != nil {
			sklog.Fatal(err)
		}
	}

}
