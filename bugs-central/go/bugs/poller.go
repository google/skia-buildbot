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
	// Mapping of client to source to query to issues. Mirrors the DB structure but stores real issues instead of counts.
	// This will be used for emailing.
	openIssues = map[types.RecognizedClient]map[types.IssueSource]map[string][]*Issue{}
	// Mutex to access to above object.
	openIssuesMutex sync.RWMutex
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

		//////////////////// Android - IssueTracker ////////////////////
		androidQueryConfig := IssueTrackerQueryConfig{
			Query:  "componentid:1346 status:open",
			Client: AndroidClient,
		}
		itAndroidIssues, itAndroidUnassignedCount, err := p.issueTrackerClient.Search(ctx, androidQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling issuetracker: %s", err)
			return
		}
		sklog.Infof("Android IssueTracker issues open:%d unassigned:%d", len(itAndroidIssues), itAndroidUnassignedCount)
		// addToOpenIssues(AndroidClient, IssueTrackerSource, itAndroidIssues)
		if err := p.issueTrackerClient.PutInDB(ctx, androidQueryConfig, len(itAndroidIssues), itAndroidUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting issuetracker into DB: %s", err)
			return
		}

		//////////////////// Flutter_on_web - Github ////////////////////
		flutterOnWebQueryConfig := GithubQueryConfig{
			Labels:           []string{"e: web_canvaskit"},
			Open:             true,
			PriorityRequired: true,
			Client:           FlutterOnWebClient,
		}
		ghFlutterOnWebIssues, ghFlutterOnWebIssuesUnassignedCount, err := p.githubClient.Search(ctx, flutterOnWebQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling github: %s", err)
			return
		}
		sklog.Infof("Flutter_on_web Github issues open:%d unassigned:%d", len(ghFlutterOnWebIssues), ghFlutterOnWebIssuesUnassignedCount)
		// addToOpenIssues(FlutterOnWebClient, GithubSource, ghFlutterOnWebIssues)
		if err := p.githubClient.PutInDB(ctx, flutterOnWebQueryConfig, len(ghFlutterOnWebIssues), ghFlutterOnWebIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting github into DB: %s", err)
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
		ghFlutterNativeIssues, ghFlutterNativeIssuesUnassignedCount, err := p.githubClient.Search(ctx, flutterNativeQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling github: %s", err)
			return
		}
		sklog.Infof("Flutter_native Github issues open:%d unassigned:%d", len(ghFlutterNativeIssues), ghFlutterNativeIssuesUnassignedCount)
		// addToOpenIssues(FlutterNativeClient, GithubSource, ghFlutterNativeIssues)
		if err := p.githubClient.PutInDB(ctx, flutterNativeQueryConfig, len(ghFlutterNativeIssues), ghFlutterNativeIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting github into DB: %s", err)
			return
		}

		//////////////////// Chromium:Internals>Skia - Monorail ////////////////////
		crQueryConfig1 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia",
			Client:   ChromiumClient,
		}
		mSkiaCrIssues, mSkiaCrIssuesUnassignedCount, err := p.monorailClient.Search(ctx, crQueryConfig1)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		sklog.Infof("Chromium:Internals>Skia Monorail issues open:%d unassigned:%d", len(mSkiaCrIssues), mSkiaCrIssuesUnassignedCount)
		// addToOpenIssues(ChromiumClient, MonorailSource, mSkiaCrIssues)
		if err := p.monorailClient.PutInDB(ctx, crQueryConfig1, len(mSkiaCrIssues), mSkiaCrIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting monorail into DB: %s", err)
			return
		}

		//////////////////// Chromium:Internals>Skia>Compositing - Monorail ////////////////////
		crQueryConfig2 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>Compositing",
			Client:   ChromiumClient,
		}
		mSkiaCompositingCrIssues, mSkiaCompositingCrIssuesUnassignedCount, err := p.monorailClient.Search(ctx, crQueryConfig2)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		sklog.Infof("Chromium:Internals>Skia>Compositing Monorail issues open:%d unassigned:%d", len(mSkiaCompositingCrIssues), mSkiaCompositingCrIssuesUnassignedCount)
		// addToOpenIssues(ChromiumClient, MonorailSource, mSkiaCompositingCrIssues)
		if err := p.monorailClient.PutInDB(ctx, crQueryConfig2, len(mSkiaCompositingCrIssues), mSkiaCompositingCrIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting monorail into DB: %s", err)
			return
		}

		//////////////////// Chromium:Internals>Skia>PDF - Monorail ////////////////////
		crQueryConfig3 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>PDF",
			Client:   ChromiumClient,
		}
		mSkiaPdfCrIssues, mSkiaPdfCrIssuesUnassignedCount, err := p.monorailClient.Search(ctx, crQueryConfig3)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		sklog.Infof("Chromium:Internals>Skia>PDF Monorail issues open:%d unassigned:%d", len(mSkiaPdfCrIssues), mSkiaPdfCrIssuesUnassignedCount)
		// addToOpenIssues(ChromiumClient, MonorailSource, mSkiaPdfCrIssues)
		if err := p.monorailClient.PutInDB(ctx, crQueryConfig3, len(mSkiaPdfCrIssues), mSkiaPdfCrIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting monorail into DB: %s", err)
			return
		}

		//////////////////// Skia - Monorail ////////////////////
		skQueryConfig := MonorailQueryConfig{
			Instance: "skia",
			Query:    "is:open",
			Client:   SkiaClient,
		}
		mSkiaSkIssues, mSkiaSkIssuesUnassignedCount, err := p.monorailClient.Search(ctx, skQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		sklog.Infof("Skia Monorail issues open:%d unassigned:%d", len(mSkiaSkIssues), mSkiaSkIssuesUnassignedCount)
		// addToOpenIssues(SkiaClient, MonorailSource, mSkiaSkIssues)
		if err := p.monorailClient.PutInDB(ctx, skQueryConfig, len(mSkiaSkIssues), mSkiaSkIssuesUnassignedCount, p.dbClient); err != nil {
			sklog.Errorf("Error putting monorail into skQueryConfig: %s", err)
			return
		}

		fmt.Println("AT THE END")
		// fmt.Printf("\n\n%+v\n\n", openIssuesClientToSource)
		// for c, frameworkToIssues := range openIssuesClientToSource {
		// 	if frameworkToIssues != nil {
		// 		for f, issues := range frameworkToIssues {
		// 			fmt.Printf("\n%s in %s has %d issues", c, f, len(issues))
		// 		}
		// 	}
		// }
		// sklog.Fatal("just testing")

	}, nil)
}

// // TODO(rmistry): This just keeps adding. It needs to populate from scratch.
// func addToOpenIssues(client types.RecognizedClient, source types.IssueSource, issues []*Issue) {
// 	openIssuesMutex.Lock()
// 	defer openIssuesMutex.Unlock()

// 	if frameworkToIssues, ok := openIssuesClientToSource[client]; ok {
// 		// Add issues to the existing slice.
// 		frameworkToIssues[source] = append(frameworkToIssues[source], issues...)
// 	} else {
// 		// Create new slice and add issues.
// 		frameworkToIssues = map[types.IssueSource][]*Issue{}
// 		frameworkToIssues[source] = append(frameworkToIssues[source], issues...)
// 		openIssuesClientToSource[client] = frameworkToIssues
// 	}
// }

func putOpenIssues(client types.RecognizedClient, source types.IssueSource, query string, issues []*Issue) {
	openIssuesMutex.Lock()
	defer openIssuesMutex.Unlock()

	if sourceToQueries, ok := openIssues[client]; ok {
		if queryToIssues, ok := sourceToQueries[source]; ok {
			// Replace existing slice with new issues.
			queryToIssues[query] = issues
		} else {
			sourceToQueries[source] = map[string][]*Issue{
				query: issues,
			}
		}
	} else {
		openIssues[client] = map[types.IssueSource]map[string][]*Issue{
			source: map[string][]*Issue{
				query: issues,
			},
		}
	}
}
