/*
	Frontend server for interacting with the AutoRoller.
*/

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/rpc"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/unthrottle"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	// flags.
	configsContents     = common.NewMultiStringFlag("config", nil, "Base 64 encoded config in JSON format. Supply this flag once for each roller. Mutually exclusive with --config_file and --config_dir.")
	configFiles         = common.NewMultiStringFlag("config_file", nil, "Path to autoroller config file. Supply this flag once for each roller. Mutually exclusive with --config and --config_dir.")
	configDir           = flag.String("config_dir", "", "Path to directory containing autoroll config files.  Mutually exclusive with --config and --config_file.")
	configGerritProject = flag.String("config_gerrit_project", "", "Gerrit project used for editing configs.")
	configRepo          = flag.String("config_repo", "", "Repo URL where configs are stored.")
	configRepoPath      = flag.String("config_repo_path", "", "Path within the config repo where configs are stored.")
	firestoreInstance   = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	host                = flag.String("host", "localhost", "HTTP service host")
	internal            = flag.Bool("internal", false, "If true, display the internal rollers.")
	local               = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port                = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort            = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir        = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	hang                = flag.Bool("hang", false, "If true, don't spin up the server, just hang without doing anything.")

	allowedViewers = []string{
		"prober@skia-public.iam.gserviceaccount.com",
		"skia-status@skia-public.iam.gserviceaccount.com",
		"skia-status-internal@skia-corp.google.com.iam.gserviceaccount.com",
		"status@skia-buildbots.google.com.iam.gserviceaccount.com",
		"status-internal@skia-buildbots.google.com.iam.gserviceaccount.com",
		"showy-dashboards@prod.google.com",
	}

	mainTemplate   *template.Template = nil
	rollerTemplate *template.Template = nil
	configTemplate *template.Template = nil

	rollerConfigs map[string]*config.Config

	configEditsInProgress               = map[string]*config.Config{}
	configGitiles         *gitiles.Repo = nil

	// gerritOauthConfig is the OAuth 2.0 client configuration used for
	// interacting with Gerrit.
	gerritOauthConfig = &oauth2.Config{
		ClientID:     "not-a-valid-client-id",
		ClientSecret: "not-a-valid-client-secret",
		Scopes:       []string{gerrit.AuthScope},
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8000/oauth2callback/",
	}
)

func reloadTemplates() {
	if *resourcesDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			sklog.Fatal(err)
		}
		*resourcesDir = filepath.Join(wd, "dist")
	}
	sklog.Infof("Reading resources from %s", *resourcesDir)
	mainTemplate = template.Must(template.New("index.html").Funcs(map[string]interface{}{
		"marshal": func(data interface{}) template.JS {
			b, _ := json.Marshal(data)
			return template.JS(b)
		},
	}).ParseFiles(
		filepath.Join(*resourcesDir, "index.html"),
	))
	rollerTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "roller.html"),
	))
	configTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "config.html"),
	))
}

func getRoller(w http.ResponseWriter, r *http.Request) *config.Config {
	name, ok := mux.Vars(r)["roller"]
	if !ok {
		http.Error(w, "Unable to find roller name in request path.", http.StatusBadRequest)
		return nil
	}
	roller, ok := rollerConfigs[name]
	if !ok {
		http.Error(w, "No such roller", http.StatusNotFound)
		return nil
	}
	return roller
}

func rollerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	cfg := getRoller(w, r)
	if cfg == nil {
		return // Errors are handled by getRoller.
	}
	page := struct {
		ChildName  string
		ParentName string
		Roller     string
	}{
		ChildName:  cfg.ChildDisplayName,
		ParentName: cfg.ParentDisplayName,
		Roller:     cfg.RollerName,
	}
	if err := rollerTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := mainTemplate.Execute(w, nil); err != nil {
		httputils.ReportError(w, errors.New("Failed to expand template."), fmt.Sprintf("Failed to expand template: %s", err), http.StatusInternalServerError)
	}
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Parse and validate the config.
		configJson := r.FormValue("configJson")
		var cfg config.Config
		if err := protojson.Unmarshal([]byte(configJson), &cfg); err != nil {
			httputils.ReportError(w, err, "Failed to parse config as JSON", http.StatusBadRequest)
			return
		}
		if err := cfg.Validate(); err != nil {
			httputils.ReportError(w, err, err.Error(), http.StatusBadRequest)
			return
		}

		// We're going to redirect for the OAuth2 flow. Store the config in
		// memory.
		// TODO(borenet): What happens if we scale Kubernetes up to multiple
		// frontend pods and the user redirects back to a different instance?
		var sessionID string
		for {
			sessionID = uuid.New().String()
			if _, ok := configEditsInProgress[sessionID]; !ok {
				break
			}
		}
		configEditsInProgress[sessionID] = &cfg
		time.AfterFunc(time.Hour, func() {
			delete(configEditsInProgress, sessionID)
		})

		// Redirect for OAuth2.
		opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("approval_prompt", "auto")}
		redirectURL := gerritOauthConfig.AuthCodeURL(sessionID, opts...)
		http.Redirect(w, r, redirectURL, http.StatusFound)
	} else {
		w.Header().Set("Content-Type", "text/html")
		if err := configTemplate.Execute(w, nil); err != nil {
			httputils.ReportError(w, errors.New("Failed to expand template."), fmt.Sprintf("Failed to expand template: %s", err), http.StatusInternalServerError)
		}
	}
}

func configJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cfg := getRoller(w, r)
	if cfg == nil {
		return // Errors are handled by getRoller.
	}

	b, err := protojson.Marshal(cfg)
	if err != nil {
		httputils.ReportError(w, err, "Failed to encode response.", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(b); err != nil {
		httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
		return
	}
}

func submitConfigUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sessionID := r.FormValue("state")
	cfg, ok := configEditsInProgress[sessionID]
	if !ok {
		msg := "Unable to find config"
		httputils.ReportError(w, errors.New(msg), msg, http.StatusBadRequest)
		return
	}
	content, err := prototext.MarshalOptions{
		Indent: "  ",
	}.Marshal(cfg)
	if err != nil {
		httputils.ReportError(w, err, "Failed to encode config to proto.", http.StatusInternalServerError)
		return
	}
	code := r.FormValue("code")
	token, err := gerritOauthConfig.Exchange(ctx, code)
	if err != nil {
		httputils.ReportError(w, err, "Failed to authenticate.", http.StatusInternalServerError)
		return
	}
	ts := gerritOauthConfig.TokenSource(ctx, token)
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	g, err := gerrit.NewGerrit(gerrit.GerritSkiaURL, client)
	if err != nil {
		httputils.ReportError(w, err, "Failed to initialize Gerrit API.", http.StatusInternalServerError)
		return
	}
	baseCommit, err := configGitiles.ResolveRef(ctx, git.MainBranch)
	if err != nil {
		httputils.ReportError(w, err, "Failed to find base commit.", http.StatusInternalServerError)
		return
	}
	configFile := cfg.RollerName + ".cfg"
	if *configRepoPath != "" {
		configFile = path.Join(*configRepoPath, configFile)
	}
	// TODO(borenet): Handle custom commit messages.
	ci, err := gerrit.CreateAndEditChange(ctx, g, *configGerritProject, git.MainBranch, "Update AutoRoller Config", baseCommit, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		return g.EditFile(ctx, ci, configFile, string(content))
	})
	if err != nil {
		httputils.ReportError(w, err, "Failed to create change.", http.StatusInternalServerError)
		return
	}
	redirectURL := g.Url(ci.Issue)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func oAuth2CallbackHandler(w http.ResponseWriter, r *http.Request) {
	// We share the same OAuth2 redirect URL between the normal login flow and
	// the Gerrit auth flow used for editing roller configs.  Use the presence
	// of the state variable in the configEditsInProgress map to distinguish
	// between the two.
	state := r.FormValue("state")
	if _, ok := configEditsInProgress[state]; ok {
		submitConfigUpdate(w, r)
	} else {
		login.OAuth2CallbackHandler(w, r)
	}
}

// addCorsMiddleware wraps the specified HTTP handler with a handler that applies the
// CORS specification on the request, and adds relevant CORS headers as necessary.
// This is needed for some handlers that do not have this middleware. Eg: the twirp
// handler (https://github.com/twitchtv/twirp/issues/210).
func addCorsMiddleware(handler http.Handler) http.Handler {
	corsWrapper := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		Debug:          true,
	})
	return corsWrapper.Handler(handler)
}

func runServer(ctx context.Context, serverURL string, srv http.Handler) {
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.HandleFunc("/config", configHandler)
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, oAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	rollerRouter := r.PathPrefix("/r/{roller}").Subrouter()
	rollerRouter.HandleFunc("", rollerHandler)
	rollerRouter.HandleFunc("/config", configJSONHandler)
	r.PathPrefix(rpc.AutoRollServicePathPrefix).Handler(addCorsMiddleware(srv))
	h := httputils.LoggingRequestResponse(r)
	h = httputils.XFrameOptionsDeny(h)
	if !*local {
		if *internal {
			h = login.RestrictViewer(h)
			h = login.ForceAuth(h, login.DEFAULT_OAUTH2_CALLBACK)
		}
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	common.InitWithMust(
		"autoroll-fe",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()

	reloadTemplates()

	if *hang {
		select {}
	}

	ts, err := auth.NewDefaultTokenSource(*local, auth.ScopeUserinfoEmail, auth.ScopeGerrit, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	namespace := ds.AUTOROLL_NS
	if *internal {
		namespace = ds.AUTOROLL_INTERNAL_NS
	}
	if err := ds.InitWithOpt(common.PROJECT_ID, namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}

	ctx := context.Background()

	manualRollDB, err := manual.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	throttleDB := unthrottle.NewDatastore(ctx)

	if *configRepo == "" {
		sklog.Fatal("--config_repo is required.")
	}
	if *configGerritProject == "" {
		sklog.Fatal("--config_gerrit_project is required.")
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	configGitiles = gitiles.NewRepo(*configRepo, client)

	// Read the configs for the rollers.
	if len(*configsContents) > 0 && (len(*configFiles) > 0 || *configDir != "") ||
		len(*configFiles) > 0 && (len(*configsContents) > 0 || *configDir != "") {
		sklog.Fatal("--config, --config_file, and --config_dir are mutually exclusive.")
	} else if len(*configsContents) == 0 && len(*configFiles) == 0 && *configDir == "" {
		sklog.Fatal("At least one instance of --config, --config_file, --config_dir is required.")
	}
	cfgBytes := make([][]byte, 0, len(*configsContents)+len(*configFiles))
	for _, cfgStr := range *configsContents {
		b, err := base64.StdEncoding.DecodeString(cfgStr)
		if err != nil {
			sklog.Fatalf("Failed to base64-decode config: %s\n\nbase64:\n%s", err, cfgStr)
		}
		cfgBytes = append(cfgBytes, b)
	}
	if *configDir != "" {
		files, err := os.ReadDir(*configDir)
		if err != nil {
			sklog.Fatalf("Failed to read config dir %s: %s", *configDir, err)
		}
		for _, file := range files {
			if file.Type().IsRegular() {
				*configFiles = append(*configFiles, filepath.Join(*configDir, file.Name()))
			}
		}
	}
	for _, path := range *configFiles {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			sklog.Fatalf("Failed to read config file %s: %s", path, err)
		}
		cfgBytes = append(cfgBytes, b)
	}
	cfgs := make([]*config.Config, 0, len(cfgBytes))
	for _, b := range cfgBytes {
		var cfg config.Config
		if err := prototext.Unmarshal(b, &cfg); err != nil {
			sklog.Fatalf("Failed to decode proto string: %s\n\nstring:\n%s", err, string(b))
		}
		cfgs = append(cfgs, &cfg)
	}

	// Validate the configs.
	rollerConfigs = make(map[string]*config.Config, len(cfgs))
	rollers := make(map[string]*rpc.AutoRoller, len(cfgs))
	for _, cfg := range cfgs {
		if err := cfg.Validate(); err != nil {
			sklog.Fatalf("Invalid roller config %q: %s", cfg.RollerName, err)
		}
		// Public frontend only displays public rollers, private-private.
		if *internal != cfg.IsInternal {
			sklog.Fatalf("Internal/external mismatch for %s", cfg.RollerName)
		}

		// Set up DBs for the roller.
		arbMode, err := modes.NewDatastoreModeHistory(ctx, cfg.RollerName)
		if err != nil {
			sklog.Fatal(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbMode.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		arbStatusDB := status.NewDatastoreDB()
		arbStatus, err := status.NewCache(ctx, arbStatusDB, cfg.RollerName)
		if err != nil {
			sklog.Fatal(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbStatus.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		arbStrategy, err := strategy.NewDatastoreStrategyHistory(ctx, cfg.RollerName, cfg.ValidStrategies())
		if err != nil {
			sklog.Fatal(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbStrategy.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		rollers[cfg.RollerName] = &rpc.AutoRoller{
			Cfg:      cfg,
			Mode:     arbMode,
			Status:   arbStatus,
			Strategy: arbStrategy,
		}
		rollerConfigs[cfg.RollerName] = cfg
	}

	// TODO(borenet): Use CRIA groups instead of @google.com, ie. admins are
	// "google/skia-root@google.com", editors are specified in each roller's
	// config file, and viewers are either public or @google.com.
	var viewAllow allowed.Allow
	if *internal {
		viewAllow = allowed.UnionOf(allowed.NewAllowedFromList(allowedViewers), allowed.Googlers())
	}
	editAllow := allowed.Googlers()
	adminAllow := allowed.Googlers()
	srv := rpc.NewAutoRollServer(ctx, rollers, manualRollDB, throttleDB, viewAllow, editAllow, adminAllow)
	if err != nil {
		sklog.Fatal(err)
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	login.InitWithAllow(serverURL+login.DEFAULT_OAUTH2_CALLBACK, adminAllow, editAllow, viewAllow)

	// Load the OAuth2 config information.
	_, clientID, clientSecret := login.TryLoadingFromKnownLocations()
	if clientID == "" || clientSecret == "" {
		sklog.Fatal("Failed to load OAuth2 configuration.")
	}
	gerritOauthConfig.ClientID = clientID
	gerritOauthConfig.ClientSecret = clientSecret
	gerritOauthConfig.RedirectURL = serverURL + login.DEFAULT_OAUTH2_CALLBACK

	// Create the server.
	runServer(ctx, serverURL, srv)
}
