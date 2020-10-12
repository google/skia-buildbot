package bugs

// Initializes and polls the different support issue frameworks

import (
	"context"
	"fmt"
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

	// Init db for writing out bug counts for the different clients and sources.
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

func (p *IssuesPoller) GetCounts(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) (int, int, error) {
	return p.dbClient.GetCountsFromDB(ctx, client, source, query)
}

// StartPoll polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func (p *IssuesPoller) StartPoll(pollInterval time.Duration) {

	openCounts, untriagedCounts, err := p.dbClient.GetCountsFromDB(context.Background(), "", "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	openCounts, untriagedCounts, err = p.dbClient.GetCountsFromDB(context.Background(), AndroidClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	openCounts, untriagedCounts, err = p.dbClient.GetCountsFromDB(context.Background(), ChromiumClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	openCounts, untriagedCounts, err = p.dbClient.GetCountsFromDB(context.Background(), FlutterNativeClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	openCounts, untriagedCounts, err = p.dbClient.GetCountsFromDB(context.Background(), FlutterOnWebClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	openCounts, untriagedCounts, err = p.dbClient.GetCountsFromDB(context.Background(), SkiaClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	openCounts, untriagedCounts, err = p.dbClient.GetCountsFromDB(context.Background(), AndroidClient, IssueTrackerSource, "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	openCounts, untriagedCounts, err = p.dbClient.GetCountsFromDB(context.Background(), AndroidClient, IssueTrackerSource, "componentid:1346 status:open")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	openCounts, untriagedCounts, err = p.dbClient.GetCountsFromDB(context.Background(), AndroidClient, MonorailSource, "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(openCounts)
	fmt.Println(untriagedCounts)
	// FOR TESTING TESTING TESTING
	return

	// Let this keep collecting open issues. Then the different endpoints can return various things from those issues.
	cleanup.Repeat(pollInterval, func(_ context.Context) {
		// Ignore the passed-in context; this allows us to continue running even if the
		// context is canceled due to transient errors.
		ctx := context.Background()

		//////////////////// Android - IssueTracker ////////////////////
		androidQueryConfig := IssueTrackerQueryConfig{
			Query:  "componentid:1346 status:open",
			Client: AndroidClient,
		}
		if err := p.issueTrackerClient.SearchClientAndPersist(ctx, androidQueryConfig, p.dbClient); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
		}

		//////////////////// Flutter_on_web - Github ////////////////////
		flutterOnWebQueryConfig := GithubQueryConfig{
			Labels:           []string{"e: web_canvaskit"},
			Open:             true,
			PriorityRequired: true,
			Client:           FlutterOnWebClient,
		}
		if err := p.githubClient.SearchClientAndPersist(ctx, flutterOnWebQueryConfig, p.dbClient); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
		}

		//////////////////// Flutter_native - Github ////////////////////
		flutterNativeQueryConfig := GithubQueryConfig{
			Labels:           []string{"dependency: skia"},
			ExcludeLabels:    []string{"e: web_canvaskit"}, // These issues are already covered by flutter-on-web
			Open:             true,
			PriorityRequired: false,
			Client:           FlutterNativeClient,
		}
		if err := p.githubClient.SearchClientAndPersist(ctx, flutterNativeQueryConfig, p.dbClient); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
		}

		//////////////////// Chromium:Internals>Skia - Monorail ////////////////////
		crQueryConfig1 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia",
			Client:   ChromiumClient,
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig1, p.dbClient); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
		}

		//////////////////// Chromium:Internals>Skia>Compositing - Monorail ////////////////////
		crQueryConfig2 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>Compositing",
			Client:   ChromiumClient,
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig2, p.dbClient); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
		}

		//////////////////// Chromium:Internals>Skia>PDF - Monorail ////////////////////
		crQueryConfig3 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>PDF",
			Client:   ChromiumClient,
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig3, p.dbClient); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
		}

		//////////////////// Skia - Monorail ////////////////////
		skQueryConfig := MonorailQueryConfig{
			Instance: "skia",
			Query:    "is:open",
			Client:   SkiaClient,
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, skQueryConfig, p.dbClient); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
		}

		PrettyPrintOpenIssues()
	}, nil)
}
