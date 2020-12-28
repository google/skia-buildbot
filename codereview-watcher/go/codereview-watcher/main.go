// Codereview Watcher monitors Skia's Github PRs and updates them if Gerrit
// changes have been created via Copybara. Also, closes the PRs if the
// corresponding Gerrit changes are closed (merged or abandoned).
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/oauth2"

	"go.skia.org/infra/codereview-watcher/go/db"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	local        = flag.Bool("local", false, "Set to true for local testing.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	fsNamespace  = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'codereview-watcher'")
	fsProjectID  = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
	pollInterval = flag.Duration("poll_interval", 2*time.Hour, "How often the server will poll Github for new open PRs.")

	// Mutex used to run one poller at a time. Easier to read logs that way.
	pollerMtx sync.Mutex
)

const (
	skiaTeamReviewsList = "reviews@skia.org"

	prCommentTxt = `This PR (HEAD: {{.HeadHash}}) has been imported to Gerrit for code review.

Please visit [review.skia.org/{{.GerritChangeID}}]({{.GerritURL}}) to see it. Please CC yourself to the Gerrit change.

Note:
* Skia uses only Gerrit for reviews and submitting code ([doc](https://skia.org/dev/contrib)).
* All comments are handled within Gerrit. Any comments on the GitHub PR will be ignored.
* The PR author can continue to upload commits to the branch used by the PR in order to address feedback from Gerrit.
* Once the code is ready to be merged, a maintainer will submit the change on Gerrit and skia-codereview-bot will close this PR.
* Similarly, if a change is abandoned on Gerrit, the corresponding PR will be closed with a note.
`
)

type prCommentVars struct {
	HeadHash       string
	GerritChangeID string
	GerritURL      string
}

var (
	prCommentTmpl = template.Must(template.New("prComment").Parse(prCommentTxt))
)

// startPoller does the following:
// * Queries github for all open pull requests.
// * For each pull request:
//   * Check to see if CLA is signed and the copybara importer ran.
//   * If CLA and importer ran then check to see if we have already commented on the PR.
//   * If not commented yet, then add an informative comment for the external contributor.
//   * Check the corresponding Gerrit change to see if it is closed. If it is closed:
//     * Close the PR with an informative comment pointing to the Gerrit change.
//     * If Gerrit change was abandoned then add the abandon text to the above comment.
func startPoller(ctx context.Context, githubClient *github.GitHub, gerritClient *gerrit.Gerrit, dbClient *db.FirestoreDB, repoName string) {
	liveness := metrics2.NewLiveness(fmt.Sprintf("codereview_watcher_%s", strings.ReplaceAll(repoName, "-", "_")))
	util.RepeatCtx(ctx, *pollInterval, func(ctx context.Context) {
		pollerMtx.Lock()
		defer pollerMtx.Unlock()
		sklog.Infof("--------- new round of polling for %s ---------", repoName)

		// Ignore the passed-in context; this allows us to continue running even if the
		// context is canceled due to transient errors.
		ctx = context.Background()

		pullRequests, err := githubClient.ListOpenPullRequests()
		if err != nil {
			sklog.Errorf("Error when listing open PRs: %s", err)
			return
		}
		for _, pr := range pullRequests {
			repo := pr.Base.GetRepo().GetFullName()
			prNumber := pr.GetNumber()
			prReadableStr := fmt.Sprintf("%s/%d", repo, prNumber)

			// Find out if the CLA was signed. Copybara only runs if the CLA is
			// signed but it does not hurt to double-check it here as well.
			isCLASigned := false
			// If copybara created a Gerrit URL then store it here.
			gerritURL := ""
			checks, err := githubClient.GetChecks(pr.Head.GetSHA())
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

			// Instantiate gerrit for this change.
			gerritChangeIDTokens := strings.Split(gerritURL, "/")
			gerritChangeID := gerritChangeIDTokens[len(gerritChangeIDTokens)-1]
			gerritChangeInfo, err := gerritClient.GetChange(ctx, gerritChangeID)

			// Check to see if we have already commented on this PR
			prData, err := dbClient.GetFromDB(ctx, repo, prNumber)
			if err != nil {
				sklog.Errorf("Error getting PR %s from DB: %s", prReadableStr, err)
				continue
			}
			if prData == nil || !prData.Commented {
				// Now add the comment. Inspired by https://github.com/golang/go/pull/42447
				vars := prCommentVars{
					HeadHash:       pr.Head.GetSHA(),
					GerritChangeID: gerritChangeID,
					GerritURL:      gerritURL,
				}
				var prCommentBytes bytes.Buffer
				if err := prCommentTmpl.Execute(&prCommentBytes, vars); err != nil {
					sklog.Errorf("Failed to execute prComment template: %s", err)
					continue
				}
				if err := githubClient.AddComment(prNumber, prCommentBytes.String()); err != nil {
					sklog.Errorf("Error when adding comment to PR %s: %s", prReadableStr, err)
					continue
				}

				// Add reviews@skia.org to CC list.
				// In the future maybe also add the Skia or Infra gardener here.
				if err := gerritClient.AddCC(ctx, gerritChangeInfo, []string{skiaTeamReviewsList}); err != nil {
					sklog.Errorf("Could not add %s to CC of %s: %s", skiaTeamReviewsList, gerritURL, err)
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
						msg += "\n\nReason: " + reason
					}
				} else if gerritChangeInfo.Status == gerrit.CHANGE_STATUS_MERGED {
					isMerged = true
				}

				// Add informative comment before closing.
				if err := githubClient.AddComment(prNumber, msg); err != nil {
					sklog.Errorf("Error when adding comment to PR %s: %s", prReadableStr, err)
					continue
				}
				// Close the PR and add it to the Datastore.
				if _, err := githubClient.ClosePullRequest(prNumber); err != nil {
					sklog.Errorf("Error when closing PR %s: %s", prReadableStr, err)
					continue
				}
				if err := dbClient.UpdateDB(ctx, repo, prNumber, isMerged, isAbandoned); err != nil {
					sklog.Errorf("Error updating PR %s in DB: %s", prReadableStr, err)
					continue
				}

				sklog.Infof("Closed PR %s because the corresponding Gerrit change %s was closed", prReadableStr, gerritURL)
			}
		}
		liveness.Reset()
		sklog.Info("-----------------------------------------------")
	})
}

func main() {
	common.InitWithMust("codereview-watcher", common.PrometheusOpt(promPort))

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate DB.
	dbClient, err := db.New(ctx, ts, *fsNamespace, *fsProjectID)
	if err != nil {
		sklog.Fatalf("Could not init DB: %s", err)
	}

	// Instantiate gerrit client.
	gerritClient, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, httpClient)
	if err != nil {
		sklog.Fatalf("Failed to create Gerrit client: %s", err)
	}

	// Instantiate github client for the different repos.
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

	// For skia-buildbot repo.
	githubSkiaBuildbotClient, err := github.NewGitHub(ctx, "google", "skia-buildbot", githubHttpClient)
	if err != nil {
		sklog.Fatalf("Could not instantiate github client for skia-buildbot: %s", err)
	}
	go startPoller(ctx, githubSkiaBuildbotClient, gerritClient, dbClient, "skia-buildbot")

	// For skia repo.
	githubSkiaClient, err := github.NewGitHub(ctx, "google", "skia", githubHttpClient)
	if err != nil {
		sklog.Fatalf("Could not instantiate github client for skia: %s", err)
	}
	startPoller(ctx, githubSkiaClient, gerritClient, dbClient, "skia")
}
