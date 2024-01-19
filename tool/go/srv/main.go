package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/unrolled/secure"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/tool/go/tool"
	"go.skia.org/infra/tool/go/types"
)

const (
	refreshConfigsDuration = time.Minute
	repoConfigDirectory    = "tool"
	commitMessage          = "Update tools.luci.app config."
)

// flags
var (
	host    = flag.String("host", "tools.luci.app", "HTTP service host")
	project = flag.String("project", "skia-public", "The Google Cloud project name.")
	configs = flag.String("configs", "./configs", "The directory that contains all the config files")
	repo    = flag.String("repo", "", "If supplied this will be the URL of a Git repo to checkout that contains the config files. They will be presumed to be in a subdirectory as supplied by the --configs flag.")
)

// server is the state of the server.
type server struct {
	templates             *template.Template
	alogin                alogin.Login
	configRefreshLiveness metrics2.Liveness
	gitilesRepo           gitiles.GitilesRepo
	gerritRepo            gerrit.GerritInterface
	gerritProject         string

	// Full path to the git executable.
	gitExe string

	// mutex protects tools.
	mutex sync.Mutex
	tools []byte // []Tool serialized as JSON.
}

// New implements baseapp.Constructor.
func New() (baseapp.App, error) {
	ctx := context.Background()

	gitExe, err := git.Executable(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	client, err := google.DefaultClient(ctx, auth.ScopeGerrit)
	if err != nil {
		return nil, skerr.Wrapf(err, "Creating authenticated HTTP client.")
	}
	gerritRepo, err := gerrit.NewGerrit(gerrit.GerritSkiaURL, client)
	if err != nil {
		return nil, skerr.Wrapf(err, "Creating Gerrit Client.")
	}
	gitilesRepo := gitiles.NewRepo("https://skia.googlesource.com/k8s-config", client)

	gerritProject := ""
	if *repo != "" {
		u, err := url.Parse(*repo)
		if err != nil {
			return nil, skerr.Wrapf(err, "--repo is not a valid URL: %q", *repo)
		}
		if len(u.Path) <= 1 {
			return nil, skerr.Wrapf(err, "--repo does not contain a project: %q", *repo)
		}
		gerritProject = u.Path[1:]
	}
	srv := &server{
		alogin:                proxylogin.NewWithDefaults(),
		configRefreshLiveness: metrics2.NewLiveness("tools_config_refresh"),
		gitExe:                gitExe,
		gitilesRepo:           gitilesRepo,
		gerritRepo:            gerritRepo,
		gerritProject:         gerritProject,
	}

	srv.loadTemplates()

	if err := srv.loadConfigs(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}

	return srv, nil
}

func (srv *server) StartRefreshingConfigs(ctx context.Context) {
	for range time.Tick(refreshConfigsDuration) {
		if err := srv.loadConfigs(ctx); err != nil {
			sklog.Errorf("Failed to refresh configs: %s", err)
		}
	}
}

func (srv *server) loadConfigs(ctx context.Context) error {
	var configDir string
	if *repo != "" {
		// Do a shallow checkout of the repo where the configs are found. In
		// testing this was the fastest option completing in ~1s.
		tmpDir, err := os.MkdirTemp("", "tool_config_checkout")
		if err != nil {
			return skerr.Wrap(err)
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				sklog.Error(err)
			}
		}()

		if _, err := exec.RunCwd(ctx, ".", srv.gitExe, "clone", "--depth=1", *repo, tmpDir); err != nil {
			return fmt.Errorf("cloning repo %s: %s", *repo, err)
		}

		configDir = filepath.Join(tmpDir, *configs)
	} else {
		configDir = *configs
	}
	allTools, messages, err := tool.LoadAndValidateFromFS(ctx, os.DirFS(configDir))
	if err != nil {
		return skerr.Wrapf(err, "Validation messages: %v", messages)
	}
	b, err := json.Marshal(allTools)
	if err != nil {
		return skerr.Wrap(err)
	}
	srv.mutex.Lock()
	defer srv.mutex.Unlock()
	srv.tools = b

	srv.configRefreshLiveness.Reset()

	return nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

// Serves up all the configs as serialized JSON.
func (srv *server) configHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	srv.mutex.Lock()
	defer srv.mutex.Unlock()
	_, err := w.Write(srv.tools)
	if err != nil {
		httputils.ReportError(w, err, "Failed serving configs.", http.StatusInternalServerError)
	}
}

func (srv *server) createOrUpdateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	var t tool.Tool
	var b bytes.Buffer
	if _, err := io.Copy(&b, r.Body); err != nil {
		httputils.ReportError(w, err, "Failed copy incoming JSON", http.StatusInternalServerError)
		return
	}

	// Validate that we have valid JSON.
	if err := json.Unmarshal(b.Bytes(), &t); err != nil {
		httputils.ReportError(w, err, "Failed decoding JSON", http.StatusInternalServerError)
		return
	}

	configFile := t.ID + ".json"
	if *configs != "" {
		configFile = path.Join(*configs, configFile)
	}

	baseCommit, err := srv.gitilesRepo.ResolveRef(ctx, git.MainBranch)
	if err != nil {
		httputils.ReportError(w, err, "Failed to find base commit.", http.StatusInternalServerError)
		return
	}

	// TODO(jcgregorio) Default reviewer set via command line flag?
	author := srv.alogin.LoggedInAs(r)
	ci, err := gerrit.CreateAndEditChange(ctx, srv.gerritRepo, srv.gerritProject, git.MainBranch, commitMessage, baseCommit, "", func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		if ci.Owner == nil {
			ci.Owner = &gerrit.Person{}
		}
		ci.Owner.Email = author.String()
		return g.EditFile(ctx, ci, configFile, b.String())
	})
	if err != nil {
		if ci != nil {
			if err2 := srv.gerritRepo.Abandon(ctx, ci, "Failed to create CL"); err2 != nil {
				sklog.Errorf("Failed to create CL with: %s\nAnd failed to abandon the change with: %s", err, err2)
			}
		}
		httputils.ReportError(w, err, "Failed creating CL.", http.StatusInternalServerError)
		return
	}

	resp := types.CreateOrUpdateResponse{
		URL: srv.gerritRepo.Url(ci.Issue),
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, err, "Failed writing response.", http.StatusInternalServerError)
	}
}

func (srv *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r chi.Router) {
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/_/configs", srv.configHandler)
	r.HandleFunc("/_/put", srv.createOrUpdateHandler)
	r.HandleFunc(("/_/login/status"), alogin.LoginStatusHandler(srv.alogin))
}

// See baseapp.App.
func (srv *server) AddMiddleware() []func(http.Handler) http.Handler {
	ret := []func(http.Handler) http.Handler{}
	if !*baseapp.Local {
		ret = append(ret, alogin.ForceRoleMiddleware(srv.alogin, roles.Viewer))
	}
	return ret
}

func main() {
	// Parse flags to be able to send *host to baseapp.Serve
	flag.Parse()
	baseapp.Serve(New, []string{*host})
}
