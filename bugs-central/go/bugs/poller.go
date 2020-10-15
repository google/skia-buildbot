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

	// runIds, err := p.dbClient.GetAllRecognizedRunIds(context.Background())
	// if err != nil {
	// 	sklog.Fatal(err)
	// }
	// for r, _ := range runIds {
	// 	fmt.Println(r)
	// }
	// return
	// fmt.Println("XXXXXXXXXXXX")
	// fmt.Println(time.Now().String())
	// fmt.Println(time.Now().Format(time.RFC3339Nano))
	// runId := time.Now().UTC().Format(time.RFC1123)
	// time.Parse(time.RFC1123)
	// return

	// fmt.Println(p.dbClient.StoreRunId(context.Background(), time.Now()))
	// fmt.Println(p.dbClient.IsRunIdValid(context.Background(), "abc"))
	// fmt.Println(p.dbClient.IsRunIdValid(context.Background(), "20201014T202808.479061000Z"))

	m, err := p.dbClient.GetClientsFromDB(context.Background())
	fmt.Printf("\n%+v\n", m)

	qcd, err := p.dbClient.GetCountsFromDB(context.Background(), "", "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(qcd)
	qcd, err = p.dbClient.GetCountsFromDB(context.Background(), AndroidClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(qcd)
	qcd, err = p.dbClient.GetCountsFromDB(context.Background(), ChromiumClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(qcd)
	qcd, err = p.dbClient.GetCountsFromDB(context.Background(), FlutterNativeClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(qcd)
	qcd, err = p.dbClient.GetCountsFromDB(context.Background(), FlutterOnWebClient, "", "")
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println("GOT THIS:")
	fmt.Println(qcd)
	// openCounts, untriagedCounts, _, err = p.dbClient.GetCountsFromDB(context.Background(), SkiaClient, "", "")
	// if err != nil {
	// 	sklog.Fatal(err)
	// }
	// fmt.Println("GOT THIS:")
	// fmt.Println(openCounts)
	// fmt.Println(untriagedCounts)
	// openCounts, untriagedCounts, _, err = p.dbClient.GetCountsFromDB(context.Background(), AndroidClient, IssueTrackerSource, "")
	// if err != nil {
	// 	sklog.Fatal(err)
	// }
	// fmt.Println("GOT THIS:")
	// fmt.Println(openCounts)
	// fmt.Println(untriagedCounts)
	// openCounts, untriagedCounts, _, err = p.dbClient.GetCountsFromDB(context.Background(), AndroidClient, IssueTrackerSource, "componentid:1346 status:open")
	// if err != nil {
	// 	sklog.Fatal(err)
	// }
	// fmt.Println("GOT THIS:")
	// fmt.Println(openCounts)
	// fmt.Println(untriagedCounts)
	// openCounts, untriagedCounts, _, err = p.dbClient.GetCountsFromDB(context.Background(), AndroidClient, MonorailSource, "")
	// if err != nil {
	// 	sklog.Fatal(err)
	// }
	// fmt.Println("GOT THIS:")
	// fmt.Println(openCounts)
	// fmt.Println(untriagedCounts)
	// FOR TESTING TESTING TESTING

	// qds, err := p.dbClient.GetQueryDataFromDB(context.Background(), "", "", "")
	// if err != nil {
	// 	sklog.Fatal(err)
	// }
	// fmt.Println("Query data!")
	// fmt.Printf("\n\n%+v\n\n%d", qds, len(qds))

	// // COnstruct your data for the charts thing here..
	// data := [][]interface{}{}
	// // The first row should contain column information.
	// data = append(data, []interface{}{"Date", "P0", "P1", "P2", "P3"})
	// // Populate the rest of the rows.
	// dateToCountsData := map[string]*types.IssueCountsData{}
	// for _, qd := range qds {
	// 	// https://stackoverflow.com/questions/36582789/javascript-toisostring-time-in-golang
	// 	d := qd.Created.UTC().Format("2006-01-02")
	// 	if _, ok := dateToCountsData[d]; !ok {
	// 		dateToCountsData[d] = &types.IssueCountsData{}
	// 	}
	// 	dateToCountsData[d].MergeInto(qd.CountsData)
	// }
	// for d, countsData := range dateToCountsData {
	// 	data = append(data, []interface{}{d, countsData.P0Count, countsData.P1Count, countsData.P2Count, countsData.P3Count})
	// }

	// fmt.Println("DONE WITH THE DATA:")
	// fmt.Printf("\n\n%+v\n\n", data)
	// data = append(data, []interface{}{"Date", "P0", "P1", "P2", "P3"})
	// data = append(data, []interface{}{"2020-10-01", 1, 9, 10, 12})
	// data = append(data, []interface{}{"2020-10-02", 11, 19, 100, 42})

	// return

	// Let this keep collecting open issues. Then the different endpoints can return various things from those issues.
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
			Query:  "componentid:1346 status:open",
			Client: AndroidClient,
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
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia",
			Client:   ChromiumClient,
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig1, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Chromium:Internals>Skia>Compositing - Monorail ////////////////////
		crQueryConfig2 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>Compositing",
			Client:   ChromiumClient,
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig2, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Chromium:Internals>Skia>PDF - Monorail ////////////////////
		crQueryConfig3 := MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>PDF",
			Client:   ChromiumClient,
		}
		if err := p.monorailClient.SearchClientAndPersist(ctx, crQueryConfig3, p.dbClient, runId); err != nil {
			sklog.Errorf("Error when searching and saving issues: %s", err)
			return
		}

		//////////////////// Skia - Monorail ////////////////////
		skQueryConfig := MonorailQueryConfig{
			Instance: "skia",
			Query:    "is:open",
			Client:   SkiaClient,
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
