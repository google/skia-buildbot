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

	"go.skia.org/infra/codereview-watcher/go/db"
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

	// Instantiate DB.
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatalf("Could not init DB: %s", err)
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
				continue
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
				fmt.Println(pr.GetHTMLURL())
				// fmt.Printf("\n\n%+v\n\n", pr.Base.GetRepo())
				fmt.Println(pr.Base.GetRepo().GetName())
				fmt.Println(pr.Base.GetRepo().GetFullName())

				repo := pr.Base.GetRepo().GetFullName()
				prNumber := pr.GetNumber()
				doc, err := dbClient.GetFromDB(ctx, repo, prNumber)
				if err != nil {
					sklog.Errorf("Error getting %s/%d from DB: %s", repo, prNumber, err)
					continue
				}
				fmt.Println("DOC:")
				fmt.Printf("\n%+v\n", doc)
				if err := dbClient.PutInDB(ctx, repo, prNumber, pr.GetHTMLURL(), true); err != nil {
					sklog.Errorf("Error putting %s/%d into DB: %s", repo, prNumber, err)
					continue
				}

				doc, err = dbClient.GetFromDB(ctx, repo, prNumber)
				if err != nil {
					sklog.Errorf("Error getting %s/%d from DB: %s", repo, prNumber, err)
					continue
				}
				fmt.Println("DOC:")
				fmt.Printf("\n%+v\n", doc)

				if err := dbClient.UpdateDB(ctx, repo, prNumber, true, false); err != nil {
					sklog.Errorf("Error updating %s/%d in DB: %s", repo, prNumber, err)
					continue
				}
				fmt.Println("DOC:")
				fmt.Printf("\n%+v\n", doc)

				// fmt.Printf("\n\n%+v\n\n", pr)
				sklog.Fatal("HERE")

				// Now add the comment
				msg := fmt.Sprintf(`This PR (HEAD: %v) has been imported to Gerrit for code review.

Please visit %s to see it.

Note:
* Skia uses only Gerrit for reviews and submitting code ([doc](https://skia.org/dev/contrib)).
* All comments are handled within Gerrit. Any comments on the GitHub PR will be ignored.
* The PR author can continue to upload commits to the branch used by the PR in order to address feedback from Gerrit.
* Once the code is ready to be merged, a maintainer will submit the change on Gerrit and skia-codereview-bot will close this PR.
* Similarly, if a change is abandoned on Gerrit, the corresponding PR will be closed with the note used to abandon.
`, pr.Head.GetSHA(), gerritURL)
				if err := githubSkiaBuildbotClient.AddComment(pr.GetNumber(), msg); err != nil {
					sklog.Errorf("Error when adding comment to PR %d: %s", pr.GetNumber(), err)
					continue
				}

				// ADD THIS TO DATASTORE SAYING I COMMENTED ON IT.

				// Now look at the Gerrit change to see if it is not-open.
				// If it abandoned then do it this way: https://github.com/golang/go/pull/42538
				// If it merged then do it this way: https://github.com/golang/go/pull/42261

				// ADD THIS TO DATATSTORE SAYING I CLOSED IT??
			}
		}
	}, nil)

	for {
	}
}
