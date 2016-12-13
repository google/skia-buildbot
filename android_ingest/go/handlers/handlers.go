package handlers

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/android_ingest/go/continuous"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
)

const (
	// MAX_RECENT is the largest number of recent requests that will be displayed.
	MAX_RECENT = 20
)

// Request is a record of a single POST request.
type Request struct {
	TS   string
	JSON string
}

var (
	templates *template.Template

	// mutex guards access to recent.
	mutex sync.Mutex

	// recent is just the last MAX_RECENT requests.
	recent []*Request

	resourcesDir string

	local bool

	process *continuous.Process
)

// UploadHandler handles POSTs of images to be analyzed.
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	// Parse incoming JSON.
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to read body.")
		return
	}

	var i interface{}
	if err := json.Unmarshal(b, &i); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode JSON.")
		return
	}

	// Store locally.
	mutex.Lock()
	defer mutex.Unlock()

	recent = append([]*Request{&Request{
		TS:   time.Now().UTC().String(),
		JSON: string(b),
	}}, recent...)

	// Keep track of the last N events.
	if len(recent) > MAX_RECENT {
		recent = recent[:MAX_RECENT]
	}
}

// IndexContent is the data passed to the index.html template.
type IndexContext struct {
	Recent      []*Request
	LastBuildId int64
}

// MainHandler displays the main page with the last MAX_RECENT Requests.
func MainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	user := login.LoggedInAs(r)
	if !local && user == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusTemporaryRedirect)
		return
	}
	if local {
		loadTemplates()
	}

	var lastBuildId int64 = -1
	// process is nil when testing.
	if process != nil {
		lastBuildId, _, _, _ = process.Last()
	}

	indexContent := &IndexContext{
		Recent:      recent,
		LastBuildId: lastBuildId,
	}

	mutex.Lock()
	defer mutex.Unlock()
	if err := templates.ExecuteTemplate(w, "index.html", indexContent); err != nil {
		glog.Errorf("Failed to expand template: %s", err)
	}
}

func loadTemplates() {
	templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(resourcesDir, "templates/index.html"),

		// Sub templates used by other templates.
		filepath.Join(resourcesDir, "templates/header.html"),
	))
}

func Init(dir string, isLocal bool, continuousProcess *continuous.Process) {
	resourcesDir = dir
	local = isLocal
	process = continuousProcess
	loadTemplates()
}
