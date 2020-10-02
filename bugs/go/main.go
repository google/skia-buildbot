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
	"time"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"google.golang.org/api/option"

	"go.skia.org/infra/bugs/go/bug_framework"
	"go.skia.org/infra/bugs/go/mail"
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

const ()

var (
	// Flags
	host    = flag.String("host", "bugs-central.skia.org", "HTTP service host")
	workdir = flag.String("workdir", ".", "Directory to use for scratch work.")
	// TODO(rmistry): This needs to be much higher 15 mins?
	pollInterval          = flag.Duration("poll_interval", 1*time.Minute, "How often the server will poll the different issue frameworks.")
	emailClientSecretFile = flag.String("email_client_secret_file", "/etc/bugs-central-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	emailTokenCacheFile   = flag.String("email_token_cache_file", "/etc/bugs-central-email-secrets/client_token.json", "OAuth token cache file for sending email.")
	serviceAccountFile    = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")

	// OAUTH params
	authAllowList = flag.String("auth_allowlist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")
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

	// NEED A BETTER WAY TO DO THIS FOR MULTIPLE BUG FRAMEWORKDS. BEST THING MIGHT BE A WRAPPER OVER ALL OF THEM.
	// ISSUETRACKER IS HERE
	issueTrackerFramework, err := bug_framework.InitIssueTracker(storageClient)
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
	githubFramework, err := bug_framework.InitGithub(ctx, "flutter", "flutter", pathToGithubToken)
	if err != nil {
		sklog.Fatalf("Failed to init github: %s", err)
	}

	// MONORAIL IS HERE
	monorailFramework, err := bug_framework.InitMonorail(ctx, *serviceAccountFile)
	if err != nil {
		sklog.Fatalf("Failed to init monorail: %s", err)
	}

	cleanup.Repeat(*pollInterval, func(_ context.Context) {
		// This is where the polling happens!
		// Ignore he passed-in context; this allows us to continue running even if the
		// context is canceled due to transient errors.
		ctx := context.Background()

		fmt.Println("RESULTS FOR BUGANIZER ARE")
		issueTrackerQueryConfig := bug_framework.IssueTrackerQueryConfig{QueryName: "c1346_total_open"}
		issueTrackerIssues, err := issueTrackerFramework.Search(ctx, issueTrackerQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling issuetracker: %s", err)
			return
		}
		for _, i := range issueTrackerIssues {
			fmt.Println(i.Link)
		}

		fmt.Println("RESULTS FOR GITHUB ARE")
		githubQueryConfig := bug_framework.GithubQueryConfig{
			Labels:     []string{"ask: skia"},
			Open:       true,
			UnAssigned: true,
		}
		githubIssues, err := githubFramework.Search(ctx, githubQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling github: %s", err)
			return
		}
		for _, i := range githubIssues {
			fmt.Println(i.Link)
		}

		fmt.Println("RESULTS FOR MONORAIL ARE")
		monorailQueryConfig := bug_framework.MonorailQueryConfig{
			Project: "chromium",
			// Query:   "is:open component=Internals>Skia",
			// Query: "is:open component=Internals>Skia>Compositing",
			Query: "is:open component=Internals>Skia>PDF",
		}
		// "-has:owner" will return untriaged.
		monorailIssues, err := monorailFramework.Search(ctx, monorailQueryConfig)
		if err != nil {
			sklog.Errorf("Error when polling monorail: %s", err)
			return
		}
		for _, i := range monorailIssues {
			fmt.Printf("\n%s - %s", i.Link, i.Owner)
		}
		fmt.Printf("\n\nFOUND %d issues\n\n", len(monorailIssues))

	}, nil)
	srv := &Server{}
	srv.loadTemplates()

	return srv, nil
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
