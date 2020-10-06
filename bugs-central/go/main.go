/*
	Leasing Server for Swarming Bots.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"google.golang.org/api/option"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/mail"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags
	host    = flag.String("host", "bugs-central.skia.org", "HTTP service host")
	workdir = flag.String("workdir", ".", "Directory to use for scratch work.")

	emailClientSecretFile = flag.String("email_client_secret_file", "/etc/bugs-central-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	emailTokenCacheFile   = flag.String("email_token_cache_file", "/etc/bugs-central-email-secrets/client_token.json", "OAuth token cache file for sending email.")
	serviceAccountFile    = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")
	authAllowList         = flag.String("auth_allowlist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")

	// TODO(rmistry): This needs to be much higher (maybe 15 mins)? 1m is only for testing.
	pollInterval = flag.Duration("poll_interval", 1*time.Minute, "How often the server will poll the different issue frameworks.")

	// Can all project definitations be moved to flags in the individual projects?

	// Does this need to be a map to keep issues unique?
	// openIssuesAcrossFrameworks = []bugs.Issue{}
	// Will need a mutext when accessing this.
	// Or better is to do per clients and then in UI show
	// Android (Buganizer, Monorail) - Both of the frameworks should be links that go to a URL.

	// Might need another level for components (mnoorail and buganizer) not for github...

	// A map from client to a map from bug framework to slice of issues.
	openIssuesClientToSource = map[bugs.RecognizedClient]map[bugs.IssueSource][]*bugs.Issue{}
	// Mutex to access to above object.
	openIssuesMutex sync.Mutex
)

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type ClientSecretJSON struct {
	Installed ClientConfig `json:"installed"`
}

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	// Create workdir if it does not exist.
	if err := os.MkdirAll(*workdir, 0755); err != nil {
		sklog.Fatalf("Could not create %s: %s", *workdir, err)
	}

	// Initialize mailing library.
	var cfg ClientSecretJSON
	err := util.WithReadFile(*emailClientSecretFile, func(f io.Reader) error {
		return json.NewDecoder(f).Decode(&cfg)
	})
	if err != nil {
		sklog.Fatalf("Failed to read client secrets from %q: %s", *emailClientSecretFile, err)
	}
	// Create a copy of the token cache file since mounted secrets are read-only
	// and the access token will need to be updated for the oauth2 flow.
	if !*baseapp.Local {
		fout, err := ioutil.TempFile("", "")
		if err != nil {
			sklog.Fatalf("Unable to create temp file %q: %s", fout.Name(), err)
		}
		err = util.WithReadFile(*emailTokenCacheFile, func(fin io.Reader) error {
			_, err := io.Copy(fout, fin)
			if err != nil {
				err = fout.Close()
			}
			return err
		})
		if err != nil {
			sklog.Fatalf("Failed to write token cache file from %q to %q: %s", *emailTokenCacheFile, fout.Name(), err)
		}
		*emailTokenCacheFile = fout.Name()
	}
	if err := mail.MailInit(cfg.Installed.ClientID, cfg.Installed.ClientSecret, *emailTokenCacheFile); err != nil {
		sklog.Fatalf("Failed to init mail library: %s", err)
	}

	var allow allowed.Allow
	if !*baseapp.Local {
		allow = allowed.NewAllowedFromList([]string{*authAllowList})
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	// HERE HER
	// Initialize bug frameworks.
	ctx := context.Background()
	// Don't need monorail scope below.
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_FULL_CONTROL, "https://www.googleapis.com/auth/monorail")
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		sklog.Fatalf("Failed to init storage client: %s", err)
	}

	// Init db
	if err := db.Init(ctx, ts); err != nil {
		sklog.Fatalf("Could not init db: %s", err)
	}
	if err := db.GetFromDB(ctx, bugs.FlutterNativeClient, bugs.GithubSource, "test query"); err != nil {
		sklog.Fatal(err)
	}
	sklog.Fatal("HERE")

	// NEED A BETTER WAY TO DO THIS FOR MULTIPLE BUG FRAMEWORKDS. BEST THING MIGHT BE A WRAPPER OVER ALL OF THEM.
	// ISSUETRACKER IS HERE
	issueTrackerFramework, err := bugs.InitIssueTracker(storageClient)
	if err != nil {
		sklog.Fatalf("Failed to init issuetracker: %s", err)
	}

	// GITHUB IS HERE
	pathToGithubToken := filepath.Join(github.GITHUB_TOKEN_SERVER_PATH, github.GITHUB_TOKEN_FILENAME)
	if *baseapp.Local {
		usr, err := user.Current()
		if err != nil {
			sklog.Fatal(err)
		}
		pathToGithubToken = filepath.Join(usr.HomeDir, github.GITHUB_TOKEN_FILENAME)
	}
	githubFramework, err := bugs.InitGithub(ctx, "flutter", "flutter", pathToGithubToken)
	if err != nil {
		sklog.Fatalf("Failed to init github: %s", err)
	}

	// MONORAIL IS HERE
	monorailFramework, err := bugs.InitMonorail(ctx, *serviceAccountFile)
	if err != nil {
		sklog.Fatalf("Failed to init monorail: %s", err)
	}

	// Let this keep collecting open issues. Then the different endpoints can return various things from those issues.
	cleanup.Repeat(*pollInterval, func(_ context.Context) {
		// Ignore he passed-in context; this allows us to continue running even if the
		// context is canceled due to transient errors.
		ctx := context.Background()

		// Collect open issues from the different clients.
		// Clients: Android, Flutter-native, Flutter-on-web, Chromium, Skia.

		// Might not need unassigned anywhere. Can just look for Owner == "" to determine that.
		// projectsToIssues := map[string]bugs.Issue{}  Might not need this either if you do everything in the front end?
		// uniqueOpenIssues := map[string]bugs.Issue{}
		// uniqueOpenIssues will be converted to openIssuesAcrossFrameworks at the end.

		// Get issues from issuetracker.
		fmt.Println("RESULTS FOR BUGANIZER ARE")
		issueTrackerIssues, err := issueTrackerFramework.Search(ctx, bugs.IssueTrackerQueryConfig{
			QueryName: "c1346_total_open",
			Client:    bugs.AndroidClient,
		})
		if err != nil {
			sklog.Errorf("Error when polling issuetracker: %s", err)
			return
		}
		for _, i := range issueTrackerIssues {
			fmt.Println(i.Link)
		}
		addToOpenIssues(bugs.AndroidClient, bugs.IssueTrackerSource, issueTrackerIssues)

		flutterOnWebGithubIssues, err := githubFramework.Search(ctx, bugs.GithubQueryConfig{
			Labels:           []string{"e: web_canvaskit"},
			Open:             true,
			PriorityRequired: true,
		})
		if err != nil {
			sklog.Errorf("Error when polling github: %s", err)
			return
		}
		for _, i := range flutterOnWebGithubIssues {
			fmt.Println(i.Link)
			fmt.Println(i.Priority)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(flutterOnWebGithubIssues))
		addToOpenIssues(bugs.FlutterOnWebClient, bugs.GithubSource, flutterOnWebGithubIssues)

		fmt.Println("RESULTS FOR GITHUB ARE")
		flutterNativeGithubIssues, err := githubFramework.Search(ctx, bugs.GithubQueryConfig{
			Labels:           []string{"dependency: skia"},
			ExcludeLabels:    []string{"e: web_canvaskit"}, // These issues are already covered by flutter-on-web
			Open:             true,
			PriorityRequired: false,
		})
		if err != nil {
			sklog.Errorf("Error when polling github: %s", err)
			return
		}
		for _, i := range flutterNativeGithubIssues {
			fmt.Println(i.Link)
			fmt.Println(i.Priority)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(flutterNativeGithubIssues))

		addToOpenIssues(bugs.FlutterNativeClient, bugs.GithubSource, flutterNativeGithubIssues)

		fmt.Println("RESULTS FOR MONORAIL ARE")
		// "-has:owner" will return untriaged.
		skiaCrMonorailIssues, err := monorailFramework.Search(ctx, bugs.MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia",
		})
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range skiaCrMonorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(skiaCrMonorailIssues))
		addToOpenIssues(bugs.ChromiumClient, bugs.MonorailSource, skiaCrMonorailIssues)

		skiaCompositingCrMonorailIssues, err := monorailFramework.Search(ctx, bugs.MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>Compositing",
		})
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range skiaCompositingCrMonorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(skiaCompositingCrMonorailIssues))
		addToOpenIssues(bugs.ChromiumClient, bugs.MonorailSource, skiaCompositingCrMonorailIssues)

		skiaPdfCrMonorailIssues, err := monorailFramework.Search(ctx, bugs.MonorailQueryConfig{
			Instance: "chromium",
			Query:    "is:open component=Internals>Skia>PDF",
		})
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range skiaPdfCrMonorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(skiaPdfCrMonorailIssues))
		addToOpenIssues(bugs.ChromiumClient, bugs.MonorailSource, skiaPdfCrMonorailIssues)

		skiaSkMonorailIssues, err := monorailFramework.Search(ctx, bugs.MonorailQueryConfig{
			Instance: "skia",
			Query:    "is:open",
		})
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range skiaSkMonorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(skiaSkMonorailIssues))
		addToOpenIssues(bugs.ChromiumClient, bugs.MonorailSource, skiaSkMonorailIssues)

		fmt.Println("AT THE END")
		fmt.Printf("\n\n%+v\n\n", openIssuesClientToSource)
		for c, frameworkToIssues := range openIssuesClientToSource {
			if frameworkToIssues != nil {
				for f, issues := range frameworkToIssues {
					fmt.Printf("\n%s in %s has %d issues", c, f, len(issues))
				}
			}
		}
		sklog.Fatal("just testing")

	}, nil)
	srv := &Server{}
	srv.loadTemplates()

	return srv, nil
}

func addToOpenIssues(client bugs.RecognizedClient, source bugs.IssueSource, issues []*bugs.Issue) {
	openIssuesMutex.Lock()
	defer openIssuesMutex.Unlock()

	if frameworkToIssues, ok := openIssuesClientToSource[client]; ok {
		// Add issues to the existing slice.
		frameworkToIssues[source] = append(frameworkToIssues[source], issues...)
	} else {
		// Create new slice and add issues.
		frameworkToIssues = map[bugs.IssueSource][]*bugs.Issue{}
		frameworkToIssues[source] = append(frameworkToIssues[source], issues...)
		openIssuesClientToSource[client] = frameworkToIssues
	}
}

// Server is the state of the server.
type Server struct {
	templates *template.Template
}

func (srv *Server) loadTemplates() {
	// srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
	// 	filepath.Join(*baseapp.ResourcesDir, "index.html"),
	// ))
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *Server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

// See baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	// For login/logout.
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	// All endpoints that require authentication should be added to this router.
	appRouter := mux.NewRouter()
	appRouter.HandleFunc("/", srv.indexHandler)

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	appHandler := http.Handler(appRouter)
	if !*baseapp.Local {
		appHandler = login.ForceAuth(appRouter, login.DEFAULT_REDIRECT_URL)
	}

	r.PathPrefix("/").Handler(appHandler)
}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
	return
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(New, []string{*host})
}
