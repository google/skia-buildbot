package poller

// Initializes and polls the various issue frameworks.

import (
	"context"
	"os/user"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/bugs-central/go/bugs/github"
	"go.skia.org/infra/bugs-central/go/bugs/issuetracker"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	github_lib "go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// IssuesPoller will be used to poll the different issue frameworks.
type IssuesPoller struct {
	storageClient     *storage.Client
	pathToGithubToken string

	dbClient   types.BugsDB
	openIssues *bugs.OpenIssues
}

// New returns an instance of IssuesPoller.
func New(ctx context.Context, ts oauth2.TokenSource, pathToGithubToken string, dbClient types.BugsDB) (*IssuesPoller, error) {
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to init storage client")
	}

	if *baseapp.Local {
		usr, err := user.Current()
		if err != nil {
			return nil, err
		}
		pathToGithubToken = filepath.Join(usr.HomeDir, github_lib.GITHUB_TOKEN_FILENAME)
	}

	// Instantiate the in-memory open issues object that will be passed to the different frameworks to
	// populate.
	openIssues := bugs.InitOpenIssues()

	return &IssuesPoller{
		storageClient:     storageClient,
		pathToGithubToken: pathToGithubToken,
		dbClient:          dbClient,
		openIssues:        openIssues,
	}, nil
}

// GetOpenIssues returns the bugs.OpenIssues held by this poller.
func (p *IssuesPoller) GetOpenIssues() *bugs.OpenIssues {
	return p.openIssues
}

// Start polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func (p *IssuesPoller) Start(ctx context.Context, pollInterval time.Duration) error {

	// Instantiate the bug frameworks with the different client configurations and then poll them.
	bugFrameworks := []bugs.BugFramework{}

	//////////////////// Android - IssueTracker ////////////////////
	androidQueryConfig := &issuetracker.IssueTrackerQueryConfig{
		Query:                         "componentid:1346 status:open",
		Client:                        types.AndroidClient,
		UntriagedPriorities:           []string{},
		UntriagedAliases:              []string{"skia-android-triage@google.com", "none"},
		HotlistsToExcludeForUntriaged: []int64{4595112},
	}
	androidIssueTracker, err := issuetracker.New(p.storageClient, p.openIssues, androidQueryConfig)
	if err != nil {
		return skerr.Wrapf(err, "failed to init issuetracker for android")
	}
	bugFrameworks = append(bugFrameworks, androidIssueTracker)

	//////////////////// Flutter_on_web - Github ////////////////////
	flutterOnWebQueryConfig := &github.GithubQueryConfig{
		Labels:           []string{"e: web_canvaskit"},
		Open:             true,
		PriorityRequired: true,
		Client:           types.FlutterOnWebClient,
	}
	flutterOnWebGithub, err := github.New(ctx, "flutter", "flutter", p.pathToGithubToken, p.openIssues, flutterOnWebQueryConfig)
	if err != nil {
		return skerr.Wrapf(err, "failed to init github for flutter-on-web")
	}
	bugFrameworks = append(bugFrameworks, flutterOnWebGithub)

	//////////////////// Flutter_native - Github ////////////////////
	flutterNativeQueryConfig := &github.GithubQueryConfig{
		Labels:           []string{"dependency: skia"},
		ExcludeLabels:    []string{"e: web_canvaskit"}, // These issues are already covered by flutter-on-web
		Open:             true,
		PriorityRequired: false,
		Client:           types.FlutterNativeClient,
	}
	flutterNativeGithub, err := github.New(ctx, "flutter", "flutter", p.pathToGithubToken, p.openIssues, flutterNativeQueryConfig)
	if err != nil {
		return skerr.Wrapf(err, "failed to init github for flutter-on-web")
	}
	bugFrameworks = append(bugFrameworks, flutterNativeGithub)

	//////////////////// Chromium - IssueTracker ////////////////////
	crQueryConfig := &issuetracker.IssueTrackerQueryConfig{
		Query:                         "componentid:1457031+ status:open",
		Client:                        types.ChromiumClient,
		UnassignedIsUntriaged:         true,
		UntriagedPriorities:           []string{},
		UntriagedAliases:              []string{"none"},
		HotlistsToExcludeForUntriaged: []int64{5438642},
	}
	crIssueTracker, err := issuetracker.New(p.storageClient, p.openIssues, crQueryConfig)
	if err != nil {
		return skerr.Wrapf(err, "failed to init issuetracker for chromium")
	}
	bugFrameworks = append(bugFrameworks, crIssueTracker)

	//////////////////// Skia - Buganizer ////////////////////
	skiaIssueTrackerQueryConfig := &issuetracker.IssueTrackerQueryConfig{
		Query:                         "componentid:1363359+ status:open -componentid:1389238+ -componentid:1399322+",
		Client:                        types.SkiaClient,
		UnassignedIsUntriaged:         true,
		UntriagedPriorities:           []string{},
		UntriagedAliases:              []string{"none"},
		HotlistsToIncludeForUntriaged: []int64{5437934},
	}
	skiaIssueTracker, err := issuetracker.New(p.storageClient, p.openIssues, skiaIssueTrackerQueryConfig)
	if err != nil {
		return skerr.Wrapf(err, "failed to init issuetracker for skia")
	}
	bugFrameworks = append(bugFrameworks, skiaIssueTracker)

	//////////////////// OSS-Fuzz - Buganizer ////////////////////
	fuzzQueryConfig := &issuetracker.IssueTrackerQueryConfig{
		Query:                 "componentid:1638179+ customfield1349507:Skia status:open",
		Client:                types.OSSFuzzClient,
		UnassignedIsUntriaged: true,
		UntriagedPriorities:   []string{},
		UntriagedAliases:      []string{"none"},
	}
	fuzzIssueTracker, err := issuetracker.New(p.storageClient, p.openIssues, fuzzQueryConfig)
	if err != nil {
		return skerr.Wrapf(err, "failed to init issuetracker for oss-fuzz")
	}
	bugFrameworks = append(bugFrameworks, fuzzIssueTracker)

	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		if !*baseapp.Local {
			// Ignore the passed-in context; this allows us to continue running even if the
			// context is canceled due to transient errors.
			ctx = context.Background()
		}

		// Create a runID timestamp to associate all found issues with this poll iteration.
		runId := p.dbClient.GenerateRunId(time.Now())

		// Search all bug frameworks.
		for _, b := range bugFrameworks {
			if err := b.SearchClientAndPersist(ctx, p.dbClient, runId); err != nil {
				sklog.Errorf("Error when searching and saving issues: %s", err)
				return
			}
		}

		// We are done with this iteration. Add the runId timestamp to the DB.
		if err := p.dbClient.StoreRunId(context.Background(), runId); err != nil {
			sklog.Errorf("Could not store runId in DB: %s", err)
			return
		}

		p.openIssues.PrettyPrintOpenIssues()
	}, nil)

	return nil
}
