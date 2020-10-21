package bugs

// Initializes and polls the various issue frameworks.

import (
	"context"
	"os/user"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// All recognized clients.
	AndroidClient       types.RecognizedClient = "Android"
	ChromiumClient      types.RecognizedClient = "Chromium"
	FlutterNativeClient types.RecognizedClient = "Flutter-native"
	FlutterOnWebClient  types.RecognizedClient = "Flutter-on-web"
	SkiaClient          types.RecognizedClient = "Skia"
)

type IssuesPoller struct {
	issueTrackerClient BugFramework
	githubClient       BugFramework
	monorailClient     BugFramework

	dbClient *db.FirestoreDB
}

func InitPoller(ctx context.Context, ts oauth2.TokenSource, serviceAccountFile string) (*IssuesPoller, error) {
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to init storage client")
	}

	// Init issuetracker.
	issueTrackerClient, err := InitIssueTracker(storageClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to init issuetracker")
	}

	// Init github.
	pathToGithubToken := filepath.Join(github.GITHUB_TOKEN_SERVER_PATH, github.GITHUB_TOKEN_FILENAME)
	if *baseapp.Local {
		usr, err := user.Current()
		if err != nil {
			return nil, err
		}
		pathToGithubToken = filepath.Join(usr.HomeDir, github.GITHUB_TOKEN_FILENAME)
	}
	githubClient, err := InitGithub(ctx, "flutter", "flutter", pathToGithubToken)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to init github")
	}

	// Init monorail.
	monorailClient, err := InitMonorail(ctx, serviceAccountFile)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to init monorail")
	}

	// Init db for writing out bug counts for the different clients+sources+queries.
	dbClient, err := db.Init(ctx, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "fould not init DB")
	}

	return &IssuesPoller{
		issueTrackerClient: issueTrackerClient,
		githubClient:       githubClient,
		monorailClient:     monorailClient,

		dbClient: dbClient,
	}, nil
}

func (p *IssuesPoller) GetClients(ctx context.Context) (map[types.RecognizedClient]map[types.IssueSource]map[string]bool, error) {
	return p.dbClient.GetClientsFromDB(ctx)
}

func (p *IssuesPoller) GetCounts(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) (*types.IssueCountsData, error) {
	return p.dbClient.GetCountsFromDB(ctx, client, source, query)
}

func (p *IssuesPoller) GetQueryData(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) ([]*db.QueryData, error) {
	return p.dbClient.GetQueryDataFromDB(ctx, client, source, query)
}

func (p *IssuesPoller) GetAllRecognizedRunIds(ctx context.Context) (map[string]bool, error) {
	return p.dbClient.GetAllRecognizedRunIds(context.Background())
}

// StartPoll polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func (p *IssuesPoller) StartPoll(pollInterval time.Duration) {

	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		if !*baseapp.Local {
			// Ignore the passed-in context; this allows us to continue running even if the
			// context is canceled due to transient errors.
			ctx = context.Background()
		}

		// Create a runID timestamp to associate all found issues with this poll iteration.
		runId := p.dbClient.GenerateRunId(time.Now())

		//////////////////// Android - IssueTracker ////////////////////
		androidQueryConfig := IssueTrackerQueryConfig{
			Query:               "componentid:1346 status:open",
			Client:              AndroidClient,
			UntriagedPriorities: []string{"P4"},
			UntriagedAliases:    []string{"skia-android-triage@google.com"},
		}
		if err := p.issueTrackerClient.SearchClientAndPersist(ctx, androidQueryConfig, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Flutter_on_web - Github ////////////////////
		flutterOnWebQueryConfig := GithubQueryConfig{
			Labels:           []string{"e: web_canvaskit"},
			Open:             true,
			PriorityRequired: true,
			Client:           FlutterOnWebClient,
		}
		if err := p.githubClient.SearchClientAndPersist(ctx, flutterOnWebQueryConfig, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Flutter_native - Github ////////////////////
		flutterNativeQueryConfig := GithubQueryConfig{
			Labels:           []string{"dependency: skia"},
			ExcludeLabels:    []string{"e: web_canvaskit"}, // These issues are already covered by flutter-on-web
			Open:             true,
			PriorityRequired: false,
			Client:           FlutterNativeClient,
		}
		if err := p.githubClient.SearchClientAndPersist(ctx, flutterNativeQueryConfig, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Chromium:Internals>Skia - Monorail ////////////////////
		crQueryConfig1 := MonorailQueryConfig{
			Instance:          "chromium",
			Query:             "is:open component=Internals>Skia",
			Client:            ChromiumClient,
			UntriagedStatuses: []string{"Untriaged", "Unconfirmed"},
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig1, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Chromium:Internals>Skia>Compositing - Monorail ////////////////////
		crQueryConfig2 := MonorailQueryConfig{
			Instance:          "chromium",
			Query:             "is:open component=Internals>Skia>Compositing",
			Client:            ChromiumClient,
			UntriagedStatuses: []string{"Untriaged", "Unconfirmed"},
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig2, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Chromium:Internals>Skia>PDF - Monorail ////////////////////
		crQueryConfig3 := MonorailQueryConfig{
			Instance:          "chromium",
			Query:             "is:open component=Internals>Skia>PDF",
			Client:            ChromiumClient,
			UntriagedStatuses: []string{"Untriaged", "Unconfirmed"},
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig3, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Skia - Monorail ////////////////////
		skQueryConfig := MonorailQueryConfig{
			Instance:          "skia",
			Query:             "is:open",
			Client:            SkiaClient,
			UntriagedStatuses: []string{"New"},
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, skQueryConfig, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		// We are done with this iteration. Add the runId timestamp to the DB.
		if err := p.dbClient.StoreRunId(context.Background(), runId); err != nil {
			sklog.Errorf("Could not store runId in DB: %s", err)
			return
		}

		PrettyPrintOpenIssues()
	}, nil)
}
