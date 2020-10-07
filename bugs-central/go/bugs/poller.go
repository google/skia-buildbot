package bugs

// Initializes and polls the different support issue frameworks

import (
	"context"
	"fmt"
	"os/user"
	"path/filepath"
	"sync"
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

var (
	// A map from client to a map from bug framework to slice of issues.
	// This will be used for emailing.
	openIssuesClientToSource = map[types.RecognizedClient]map[types.IssueSource][]*Issue{}
	// Mutex to access to above object.
	openIssuesMutex sync.Mutex
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

// Poll hits the different issue frameworks and populates DB and the other object with that data.
// It hardcodes information about our various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future..
func (p *IssuesPoller) StartPoll(pollInterval time.Duration) {
	// Let this keep collecting open issues. Then the different endpoints can return various things from those issues.
	cleanup.Repeat(pollInterval, func(_ context.Context) {
		// Ignore the passed-in context; this allows us to continue running even if the
		// context is canceled due to transient errors.
		ctx := context.Background()

		// Get issues from issuetracker.
		// ISSE TRACKER ONLY NEEDS SLICE OF QUERIES
		androidQueryConfig := IssueTrackerQueryConfig{
			Query:  "componentid:1346 status:open",
			Client: AndroidClient,
		}
		issueTrackerIssues, issueTrackerIssuesUnassignedCount, err := p.issueTrackerClient.Search(ctx, androidQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling issuetracker: %s", err)
			return
		}
		for _, i := range issueTrackerIssues {
			fmt.Println(i.Link)
		}
		addToOpenIssues(AndroidClient, IssueTrackerSource, issueTrackerIssues)
		fmt.Println("PUTTING INTO DB")
		if err := p.issueTrackerClient.PutInDB(ctx, androidQueryConfig, len(issueTrackerIssues), issueTrackerIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting into DB: %s", err)
			return
		}
		fmt.Println("DONE PUTTING INTO DB")

		// GITHUB NEEDS - OWNER/REPO/LABELS/PRIORITYREQUIRED/EXPLUCDE_LABELS - lots of stuff..
		flutterOnWebQueryConfig := GithubQueryConfig{
			Labels:           []string{"e: web_canvaskit"},
			Open:             true,
			PriorityRequired: true,
			Client:           FlutterOnWebClient,
		}
		flutterOnWebGithubIssues, flutterOnWebGithubIssuesUnassignedCount, err := p.githubClient.Search(ctx, flutterOnWebQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling github: %s", err)
			return
		}
		for _, i := range flutterOnWebGithubIssues {
			fmt.Println(i.Link)
			fmt.Println(i.Priority)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(flutterOnWebGithubIssues))
		addToOpenIssues(FlutterOnWebClient, GithubSource, flutterOnWebGithubIssues)
		if err := p.githubClient.PutInDB(ctx, flutterOnWebQueryConfig, len(flutterOnWebGithubIssues), flutterOnWebGithubIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting into DB: %s", err)
			return
		}

		fmt.Println("RESULTS FOR GITHUB ARE")
		flutterNativeQueryConfig := GithubQueryConfig{
			Labels:           []string{"dependency: skia"},
			ExcludeLabels:    []string{"e: web_canvaskit"}, // These issues are already covered by flutter-on-web
			Open:             true,
			PriorityRequired: false,
			Client:           FlutterNativeClient,
		}
		flutterNativeGithubIssues, flutterNativeGithubIssuesUnassignedCount, err := p.githubClient.Search(ctx, flutterNativeQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling github: %s", err)
			return
		}
		for _, i := range flutterNativeGithubIssues {
			fmt.Println(i.Link)
			fmt.Println(i.Priority)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(flutterNativeGithubIssues))

		addToOpenIssues(FlutterNativeClient, GithubSource, flutterNativeGithubIssues)
		if err := p.githubClient.PutInDB(ctx, flutterNativeQueryConfig, len(flutterNativeGithubIssues), flutterNativeGithubIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting into DB: %s", err)
			return
		}

		// MNOORAIL NEEDS instance and query, that's it.
		fmt.Println("RESULTS FOR MONORAIL ARE")
		// "-has:owner" will return untriaged.
		qc1 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia",
		}
		skiaCrMonorailIssues, skiaCrMonorailIssuesUnassignedCount, err := p.monorailClient.Search(ctx, qc1)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range skiaCrMonorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(skiaCrMonorailIssues))
		addToOpenIssues(ChromiumClient, MonorailSource, skiaCrMonorailIssues)
		if err := p.monorailClient.PutInDB(ctx, qc1, len(skiaCrMonorailIssues), skiaCrMonorailIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting into DB: %s", err)
			return
		}

		qc2 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>Compositing",
		}
		skiaCompositingCrMonorailIssues, skiaCompositingCrMonorailIssuesUnassignedCount, err := p.monorailClient.Search(ctx, qc2)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range skiaCompositingCrMonorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(skiaCompositingCrMonorailIssues))
		addToOpenIssues(ChromiumClient, MonorailSource, skiaCompositingCrMonorailIssues)
		if err := p.monorailClient.PutInDB(ctx, qc2, len(skiaCompositingCrMonorailIssues), skiaCompositingCrMonorailIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting into DB: %s", err)
			return
		}

		qc3 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>PDF",
		}
		skiaPdfCrMonorailIssues, skiaPdfCrMonorailIssuesUnassignedCount, err := p.monorailClient.Search(ctx, qc3)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range skiaPdfCrMonorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(skiaPdfCrMonorailIssues))
		addToOpenIssues(ChromiumClient, MonorailSource, skiaPdfCrMonorailIssues)
		if err := p.monorailClient.PutInDB(ctx, qc3, len(skiaPdfCrMonorailIssues), skiaPdfCrMonorailIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting into DB: %s", err)
			return
		}

		qc4 := MonorailQueryConfig{
			Instance: "skia",
			Query:    "is:open",
		}
		skiaSkMonorailIssues, skiaSkMonorailIssuesUnassignedCount, err := p.monorailClient.Search(ctx, qc4)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range skiaSkMonorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(skiaSkMonorailIssues))
		addToOpenIssues(SkiaClient, MonorailSource, skiaSkMonorailIssues)
		if err := p.monorailClient.PutInDB(ctx, qc4, len(skiaSkMonorailIssues), skiaSkMonorailIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting into DB: %s", err)
			return
		}

		fmt.Println("AT THE END")
		fmt.Printf("\n\n%+v\n\n", openIssuesClientToSource)
		for c, frameworkToIssues := range openIssuesClientToSource {
			if frameworkToIssues != nil {
				for f, issues := range frameworkToIssues {
					fmt.Printf("\n%s in %s has %d issues", c, f, len(issues))
				}
			}
		}
		// sklog.Fatal("just testing")

	}, nil)
}

// TODO(rmistry): This just keeps adding. It needs to populate from scratch.
func addToOpenIssues(client types.RecognizedClient, source types.IssueSource, issues []*Issue) {
	openIssuesMutex.Lock()
	defer openIssuesMutex.Unlock()

	if frameworkToIssues, ok := openIssuesClientToSource[client]; ok {
		// Add issues to the existing slice.
		frameworkToIssues[source] = append(frameworkToIssues[source], issues...)
	} else {
		// Create new slice and add issues.
		frameworkToIssues = map[types.IssueSource][]*Issue{}
		frameworkToIssues[source] = append(frameworkToIssues[source], issues...)
		openIssuesClientToSource[client] = frameworkToIssues
	}
}
