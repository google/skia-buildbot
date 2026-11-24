// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/cd/go/deployment_status"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
)

const expectRepoFlagFormat = "<bt_project>/<bt_instance>:<repo1>[,repo2]..."

type btRepoSpec struct {
	project  string
	instance string
	repos    []string
}

type btRepoSpecs []*btRepoSpec

func (s *btRepoSpecs) String() string {
	var rv strings.Builder
	for _, spec := range *s {
		_, _ = rv.WriteString(fmt.Sprintf("%s/%s:%s;", spec.project, spec.instance, strings.Join(spec.repos, ",")))
	}
	return rv.String()
}

func (s *btRepoSpecs) Set(value string) error {
	split := strings.SplitN(value, "/", 2)
	if len(split) == 2 && split[0] != "" {
		instanceAndRepos := strings.SplitN(split[1], ":", 2)
		if len(instanceAndRepos) == 2 && instanceAndRepos[0] != "" && instanceAndRepos[1] != "" {
			*s = append(*s, &btRepoSpec{
				project:  split[0],
				instance: instanceAndRepos[0],
				repos:    strings.Split(instanceAndRepos[1], ","),
			})
			return nil
		}
	}
	return skerr.Fmt("expected %q, not %q", expectRepoFlagFormat, value)
}

var _ flag.Value = (*btRepoSpecs)(nil)

func btRepoSpecsFlag(name, usage string) *[]*btRepoSpec {
	var values []*btRepoSpec
	flag.Var((*btRepoSpecs)(&values), name, usage)
	return &values
}

func main() {
	btAppProfile := flag.String("bigtable_app_profile", "default", "BigTable app profile to use.")
	gitstoreTable := flag.String("gitstore_bt_table", "git-repos2", "BigTable table used for GitStore.")
	port := flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	repoSpecs := btRepoSpecsFlag("repo", fmt.Sprintf("Maps BigTable project and instance to git repository URLs, eg. %q.", expectRepoFlagFormat))
	resourcesDir := flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files.")
	flag.Parse()

	if len(*repoSpecs) == 0 {
		sklog.Fatal("At least one --repo is required.")
	}
	if *resourcesDir == "" {
		sklog.Fatal("--resources_dir is required.")
	}

	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, bigtable.Scope)
	if err != nil {
		sklog.Fatal(err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	d, err := docker.NewClient(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	repos := repograph.Map{}
	var allRepoURLs []string
	for _, spec := range *repoSpecs {
		btConf := &bt_gitstore.BTConfig{
			ProjectID:  spec.project,
			InstanceID: spec.instance,
			TableID:    *gitstoreTable,
			AppProfile: *btAppProfile,
		}
		specRepos, err := bt_gitstore.NewBTGitStoreMap(ctx, spec.repos, btConf)
		if err != nil {
			sklog.Fatal(err)
		}
		for k, v := range specRepos {
			repos[k] = v
		}
		allRepoURLs = append(allRepoURLs, spec.repos...)
	}

	gitRepos := make([]deployment_status.GitRepo, 0, len(allRepoURLs))
	for _, repoURL := range allRepoURLs {
		gr, err := gitiles.NewRepoWithClient(repoURL, c)
		if err != nil {
			sklog.Fatal(err)
		}
		gitRepos = append(gitRepos, deployment_status.NewRepographRepo(gr, repos[repoURL]))
	}

	checker, err := deployment_status.NewChecker(ctx, c, d, gitRepos)
	if err != nil {
		if checker == nil {
			sklog.Fatal(err)
		}
		sklog.Errorf("Initial entry refresh completed with errors: %s", err)
	}
	checker.StartAutoRefresh(ctx, time.Minute)

	http.HandleFunc("/api/entries", func(w http.ResponseWriter, r *http.Request) {
		resp, err := checker.QueryEntries(r.Context(), r.URL.Query())
		if err != nil {
			httputils.ReportError(w, err, "Failed to query entries", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			sklog.Errorf("Failed to encode entries: %s", err)
		}
	})

	resourceHandler := httputils.MakeResourceHandler(*resourcesDir)
	http.Handle("/dist/", http.StripPrefix("/dist/", http.HandlerFunc(resourceHandler)))
	http.HandleFunc("/", resourceHandler)

	sklog.Infof("Listening on http://localhost%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
