// Codereview Watcher monitors the Skia's Github PRs.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	local    = flag.Bool("local", false, "Set to true for local testing.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")

	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'codereview-watcher'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	pollInterval = flag.Duration("poll_interval", 2*time.Hour, "How often the server will poll Github for new open PRs.")
)

func main() {
	common.InitWithMust("codereview-watcher", common.PrometheusOpt(promPort))

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate gerrit client.
	gerritClient, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, httpClient)
	if err != nil {
		sklog.Fatalf("Failed to create Gerrit client: %s", err)
	}
	fmt.Println(gerritClient)

	// Instantiate github client.
	pathToGithubToken := filepath.Join(github.GITHUB_TOKEN_SERVER_PATH, github.GITHUB_TOKEN_FILENAME)
	if *local {
		usr, err := user.Current()
		if err != nil {
			sklog.Fatal(err)
		}
		pathToGithubToken = filepath.Join(usr.HomeDir, github.GITHUB_TOKEN_FILENAME)
	}
	gBody, err := ioutil.ReadFile(pathToGithubToken)
	if err != nil {
		sklog.Fatalf("Could not find githubToken in %s: %s", pathToGithubToken, err)
	}
	gToken := strings.TrimSpace(string(gBody))
	githubTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})
	githubHttpClient := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(githubTS).Client()

	// CREATE ONE FOR SKIA AS WELL!
	githubSkiaBuildbotClient, err := github.NewGitHub(ctx, "google", "skia-buildbot", githubHttpClient)
	if err != nil {
		sklog.Fatalf("Could not instantiate github client: %s", err)
	}

	cleanup.Repeat(*pollInterval, func(ctx context.Context) {
		// Ignore the passed-in context; this allows us to continue running even if the
		// context is canceled due to transient errors.
		ctx = context.Background()

		pullRequests, err := githubSkiaBuildbotClient.ListOpenPullRequests()
		if err != nil {
			sklog.Errorf("Error when listing open PRs: %s", err)
			return
		}
		for _, pr := range pullRequests {
			// Find out if the CLA was signed. Copybara only runs if the CLA is signed but it does not
			// hurt to double-check it here as well.
			isCLASigned := false
			// If copybara created a Gerrit URL then store it here.
			gerritURL := ""
			checks, err := githubSkiaBuildbotClient.GetChecks(pr.Head.GetSHA())
			if err != nil {
				sklog.Errorf("Error when getting checks for PR %s: %s", pr.GetID(), err)
			}

			for _, c := range checks {
				if c.State != github.CHECK_STATE_SUCCESS {
					continue
				}
				if c.Name == github.CLA_CHECK {
					isCLASigned = true
				} else if c.Name == github.IMPORT_COPYBARA_CHECK {
					gerritURL = c.HTMLURL
				}
			}

			if isCLASigned && gerritURL != "" {
				// We are interested in this PR.
				fmt.Printf("\nWE ARE INTERESTED IN %d", pr.GetNumber())

				// Now add the comment
			}
		}
	}, nil)

	for {
	}
}
