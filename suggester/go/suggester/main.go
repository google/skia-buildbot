// For a given time period, find all the bots that failed when a CL, that was
// later reverted, first landed. The count of failed bots does not include bots
// that failed at both the initial commit and at the revert. Note that "-All"
// is removed from bot names.
//
// Running this requires a client_secret.json file in the current directory that
// is good for accessing the swarming API.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/suggester/go/ds"
)

var (
	gitRepoDir   = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL   = flag.String("git_repo_url", "https://skia.googlesource.com/skia.git", "The URL to pass to git clone for the source repository.")
	namespace    = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'perf'.")
	projectName  = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")
	since        = flag.String("since", "6months", "How far back to search in git history.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

type GerritFiles map[string]interface{}

type FileCount struct {
	Counts string
}

var (
	totals = map[string]map[string]int{}
)

func add(filename, botname string) {
	if strings.TrimSpace(filename) == "" {
		return
	}
	if bots, ok := totals[filename]; !ok {
		totals[filename] = map[string]int{botname: 1}
	} else {
		bots[botname] = bots[botname] + 1
	}
}

func main() {
	defer common.LogPanic()
	common.Init()

	if *namespace == "" {
		sklog.Fatal("The --namespace flag is required. See infra/DATASTORE.md for format details.\n")
	}
	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init Cloud Datastore: %s", err)
	}

	// Check out or pull the repo.
	ctx := context.Background()
	git, err := git.NewCheckout(ctx, *gitRepoURL, *gitRepoDir)
	if err != nil {
		sklog.Fatal(err)
	}
	// Run ./bin/try --list to get the list of legal bots.

	httpClient, err := auth.NewDefaultClient(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	swarmApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}
	resp, err := swarmApi.ListTasks(time.Now().Add(-1*time.Hour), time.Now(), []string{"pool:Skia"}, "completed_failure")
	if err != nil {
		sklog.Fatal(err)
	}
	prefix := make([]byte, 5)
	for _, r := range resp {
		tags := map[string]string{}
		for _, s := range r.TaskResult.Tags {
			parts := strings.SplitN(s, ":", 2)
			tags[parts[0]] = parts[1]
		}
		if tags["sk_repo"] != *gitRepoURL {
			sklog.Info("Not a change for our selected repo.")
			continue
		}
		if tags["sk_issue_server"] != "" {
			fmt.Printf("Issue: %s, Patch: %s Name: %s\n", tags["sk_issue"], tags["sk_patchset"], tags["sk_name"])
			url := fmt.Sprintf("%s/changes/%s/revisions/%s/files/", tags["sk_issue_server"], tags["sk_issue"], tags["sk_patchset"])
			resp, err := httpClient.Get(url)
			if err != nil {
				sklog.Warningf("Failed to get commit file list from Gerrit: %s", err)
				continue
			}
			defer util.Close(resp.Body)
			// Trim off the first 5 chars.
			n, err := resp.Body.Read(prefix)
			if n != 5 || err != nil {
				sklog.Warningf("Failed to read file list from Gerrit: %s", err)
				continue
			}
			files := GerritFiles{}
			if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
				sklog.Warningf("Failed to get decode file list from Gerrit: %s", err)
				continue
			}
			for k, _ := range files {
				add(k, tags["sk_name"])
			}
		} else if tags["sk_revision"] != "" {
			fmt.Printf("Commit: %s, Name: %s\n", tags["sk_revision"], tags["sk_name"])

			files, err := git.Git(ctx, "show", "--pretty=", "--name-only", tags["sk_revision"])
			if err != nil {
				sklog.Warningf("Failed to get commit file list: %s", err)
				continue
			}
			for _, filename := range strings.Split(files, "\n") {
				add(filename, tags["sk_name"])
			}
		} else {
			fmt.Printf("Leased device task.\n")
		}

		// If sk_name is in the list of legal bots, get the list of files changed.
		// or
		//  GET JSON from https://skia-review.googlesource.com/changes/81121/revisions/8/files/
		//
		// Increment results in:
		//            map[filename]map[botname]int
		//            map[string]map[string]int
	}

	// Once all are processed then produce a dataset with most popular (or two most popular) bots
	// for each filename.
	keys := []string{}
	for k, _ := range totals {
		keys = append(keys, k)
	}
	dsKey := ds.NewKey(ds.FILE_COUNT)
	fc := &FileCount{}
	for _, key := range keys {
		fmt.Printf("%s: %v\n", key, totals[key])
		dsKey.Name = key
		b, err := json.Marshal(totals[key])
		if err != nil {
			sklog.Fatalf("Failed encoding before writing to datastore: %s", err)
		}
		fc.Counts = string(b)
		_, err = ds.DS.Put(ctx, dsKey, fc)
		if err != nil {
			sklog.Fatalf("Failed writing to datastore: %s", err)
		}
	}
}
