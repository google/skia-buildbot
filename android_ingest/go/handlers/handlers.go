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

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
)

const (
	MAX_RECENT = 20
)

type Request struct {
	TS   int64
	JSON string
}

var (
	templates *template.Template

	mutex sync.Mutex

	recent []*Request

	resourcesDir string

	local bool
)

// uploadHandler handles POSTs of images to be analyzed.
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
		TS:   time.Now().UTC().Unix(),
		JSON: string(b),
	}}, recent...)

	// Keep track of the last N events.
	if len(recent) > MAX_RECENT {
		recent = recent[:MAX_RECENT]
	}
}

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

	mutex.Lock()
	defer mutex.Unlock()
	if err := templates.ExecuteTemplate(w, "index.html", recent); err != nil {
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

func Init(dir string, isLocal bool) {
	resourcesDir = dir
	local = isLocal
	loadTemplates()
}
