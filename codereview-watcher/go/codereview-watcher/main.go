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

	// TODO(rmistry): CREATE ONE FOR SKIA AS WELL!
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
			repo := pr.Base.GetRepo().GetFullName()
			prNumber := pr.GetNumber()
			prReadableStr := fmt.Sprintf("%s/%d", repo, prNumber)

			// Find out if the CLA was signed. Copybara only runs if the CLA is signed but it does not
			// hurt to double-check it here as well.
			isCLASigned := false
			// If copybara created a Gerrit URL then store it here.
			gerritURL := ""
			checks, err := githubSkiaBuildbotClient.GetChecks(pr.Head.GetSHA())
			if err != nil {
				sklog.Errorf("Error when getting checks for PR %s: %s", prReadableStr, err)
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
			if !isCLASigned {
				sklog.Infof("CLA has not been signed yet for PR %s", prReadableStr)
				continue
			}
			if gerritURL == "" {
				sklog.Infof("Copybara importer has not run yet on PR %s", prReadableStr)
				continue
			}

			// Check to see if we have already commented on this PR
			prData, err := dbClient.GetFromDB(ctx, repo, prNumber)
			if err != nil {
				sklog.Errorf("Error getting PR %s from DB: %s", prReadableStr, err)
				continue
			}
			if prData == nil || !prData.Commented {
				// Now add the comment. Inspired by https://github.com/golang/go/pull/42447
				msg := fmt.Sprintf(`This PR (HEAD: %v) has been imported to Gerrit for code review.

Please visit %s to see it.

Note:
* Skia uses only Gerrit for reviews and submitting code ([doc](https://skia.org/dev/contrib)).
* All comments are handled within Gerrit. Any comments on the GitHub PR will be ignored.
* The PR author can continue to upload commits to the branch used by the PR in order to address feedback from Gerrit.
* Once the code is ready to be merged, a maintainer will submit the change on Gerrit and skia-codereview-bot will close this PR.
* Similarly, if a change is abandoned on Gerrit, the corresponding PR will be closed with the note used to abandon.
`, pr.Head.GetSHA(), gerritURL)
				if err := githubSkiaBuildbotClient.AddComment(prNumber, msg); err != nil {
					sklog.Errorf("Error when adding comment to PR %s: %s", prReadableStr, err)
					continue
				}

				// Add to datastore saying we commented on the PR.
				if err := dbClient.PutInDB(ctx, repo, prNumber, pr.GetHTMLURL(), true); err != nil {
					sklog.Errorf("Error putting PR %s into DB: %s", prReadableStr, err)
					continue
				}

				sklog.Infof("Added comment to PR %s", prReadableStr)
			} else {
				sklog.Infof("Already added comment to PR %s previously", prReadableStr)
			}

			// Call the Gerrit API to see if the Gerrit change is not open anymore.
			gerritChangeIDTokens := strings.Split(gerritURL, "/")
			gerritChangeID := gerritChangeIDTokens[len(gerritChangeIDTokens)-1]
			gerritChangeInfo, err := gerritClient.GetChange(ctx, gerritChangeID)
			if err != nil {
				sklog.Errorf("Error getting gerrit change %s: %s", gerritURL, err)
				continue
			}
			if gerritChangeInfo.IsClosed() {
				// Inspired by https://github.com/golang/go/pull/42261
				msg := fmt.Sprintf(`This PR is being closed because [review.skia.org/%s](%s) has been %s.`, gerritChangeID, gerritURL, strings.ToLower(gerritChangeInfo.Status))
				isAbandoned := false
				isMerged := false
				if gerritChangeInfo.Status == gerrit.CHANGE_STATUS_ABANDONED {
					isAbandoned = true
					// Inspired by https://github.com/golang/go/pull/42538
					if reason := gerritChangeInfo.GetAbandonReason(ctx); reason != "" {
						msg += "\n\n" + reason
					}
					if err := githubSkiaBuildbotClient.AddComment(prNumber, msg); err != nil {
						sklog.Errorf("Error when adding comment to PR %s: %s", prReadableStr, err)
						continue
					}
				} else if gerritChangeInfo.Status == gerrit.CHANGE_STATUS_MERGED {
					isMerged = true
				}

				// Close the PR and add it to the Datastore.
				if _, err := githubSkiaBuildbotClient.ClosePullRequest(prNumber); err != nil {
					sklog.Errorf("Error when closing PR %s: %s", prReadableStr, err)
					continue
				}
				if err := dbClient.UpdateDB(ctx, repo, prNumber, isMerged, isAbandoned); err != nil {
					sklog.Errorf("Error updating PR %s in DB: %s", prReadableStr, err)
					continue
				}
			}
		}
	}, nil)

	for {
	}
}
