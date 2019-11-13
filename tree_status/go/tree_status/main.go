package main

import (
	//"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/am/go/silence"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

// flags
var (
	accessGroup        = flag.String("access_group", "googlers", "The chrome infra auth group to use for users incidents can be assigned to.")
	modifyGroup        = flag.String("modify_group", "googlers", "The chrome infra auth group to use for restricting access.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	namespace          = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'tree-status-staging'.")
	project            = flag.String("project", "skia-public", "The Google Cloud project name.")
)

// Server is the state of the server.
type Server struct {
	incidentStore *incident.Store
	silenceStore  *silence.Store
	templates     *template.Template
	access        allowed.Allow // Who is allowed to use the site.
	modify        allowed.Allow // Who is allowed to modify data on the site.
}

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	var access allowed.Allow
	var modify allowed.Allow
	if !*baseapp.Local {
		ts, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			return nil, err
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		access, err = allowed.NewAllowedFromChromeInfraAuth(client, *accessGroup)
		if err != nil {
			return nil, err
		}
		modify, err = allowed.NewAllowedFromChromeInfraAuth(client, *modifyGroup)
		if err != nil {
			return nil, err
		}
	} else {
		access = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
		modify = allowed.NewAllowedFromList([]string{"betty@example.org", "fred@example.org", "barney@example.org", "wilma@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, access)

	//ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, pubsub.ScopePubSub, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return nil, err
	}

	if *namespace == "" {
		return nil, fmt.Errorf("The --namespace flag is required. See infra/DATASTORE.md for format details.\n")
	}
	if !*baseapp.Local && !util.In(*namespace, []string{ds.TREE_STATUS_NS}) {
		return nil, fmt.Errorf("When running in prod the datastore namespace must be a known value.")
	}
	if err := ds.InitWithOpt(*project, *namespace, option.WithTokenSource(ts)); err != nil {
		return nil, fmt.Errorf("Failed to init Cloud Datastore: %s", err)
	}

	srv := &Server{
		//treeStore: tree.NewStore,
		// Also add sheriff and the other stuff in here...
		access: access,
		modify: modify,
	}
	srv.loadTemplates()

	locations := []string{"skia-public", "google.com:skia-corp"}
	livenesses := map[string]metrics2.Liveness{}
	for _, location := range locations {
		livenesses[location] = metrics2.NewLiveness("alive", map[string]string{"location": location})
	}

	return srv, nil
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
}

func (srv *Server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

type AddNoteRequest struct {
	Text string `json:"text"`
	Key  string `json:"key"`
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *Server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

func (srv *Server) addNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req AddNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "add-note", req)

	note := note.Note{
		Text:   req.Text,
		TS:     time.Now().Unix(),
		Author: srv.user(r),
	}
	in, err := srv.incidentStore.AddNote(req.Key, note)
	if err != nil {
		httputils.ReportError(w, err, "Failed to add note.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) addSilenceNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req AddNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "add-silence-note", req)

	note := note.Note{
		Text:   req.Text,
		TS:     time.Now().Unix(),
		Author: srv.user(r),
	}
	in, err := srv.silenceStore.AddNote(req.Key, note)
	if err != nil {
		httputils.ReportError(w, err, "Failed to add note.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type DelNoteRequest struct {
	Index int    `json:"index"`
	Key   string `json:"key"`
}

func (srv *Server) delNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req DelNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "del-note", req)
	in, err := srv.incidentStore.DeleteNote(req.Key, req.Index)
	if err != nil {
		httputils.ReportError(w, err, "Failed to add note.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) delSilenceNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req DelNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "del-silence-note", req)
	in, err := srv.silenceStore.DeleteNote(req.Key, req.Index)
	if err != nil {
		httputils.ReportError(w, err, "Failed to add note.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type TakeRequest struct {
	Key string `json:"key"`
}

func (srv *Server) takeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req TakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode take request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "take", req)

	in, err := srv.incidentStore.Assign(req.Key, srv.user(r))
	if err != nil {
		httputils.ReportError(w, err, "Failed to assign.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type StatsRequest struct {
	Range string `json:"range"`
}

type Stat struct {
	Num      int               `json:"num"`
	Incident incident.Incident `json:"incident"`
}

type StatsResponse []*Stat

type StatsResponseSlice StatsResponse

func (p StatsResponseSlice) Len() int           { return len(p) }
func (p StatsResponseSlice) Less(i, j int) bool { return p[i].Num > p[j].Num }
func (p StatsResponseSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (srv *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req StatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode stats request.", http.StatusInternalServerError)
		return
	}
	ins, err := srv.incidentStore.GetRecentlyResolvedInRange(req.Range)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for Incidents.", http.StatusInternalServerError)
	}
	count := map[string]*Stat{}
	for _, in := range ins {
		if stat, ok := count[in.ID]; !ok {
			count[in.ID] = &Stat{
				Num:      1,
				Incident: in,
			}
		} else {
			stat.Num += 1
		}
	}
	ret := StatsResponse{}
	for _, v := range count {
		ret = append(ret, v)
	}
	sort.Sort(StatsResponseSlice(ret))
	if err := json.NewEncoder(w).Encode(ret); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type IncidentsInRangeRequest struct {
	Range    string            `json:"range"`
	Incident incident.Incident `json:"incident"`
}

func (srv *Server) incidentsInRangeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req IncidentsInRangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode incident range request.", http.StatusInternalServerError)
		return
	}
	ret, err := srv.incidentStore.GetRecentlyResolvedInRangeWithID(req.Range, req.Incident.ID)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for incidents.", http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(ret); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type AssignRequest struct {
	Key   string `json:"key"`
	Email string `json:"email"`
}

func (srv *Server) assignHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode take request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "assign", req)
	in, err := srv.incidentStore.Assign(req.Key, req.Email)
	if err != nil {
		httputils.ReportError(w, err, "Failed to assign.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) silencesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	silences, err := srv.silenceStore.GetAll()
	if err != nil {
		httputils.ReportError(w, err, "Failed to load recents.", http.StatusInternalServerError)
		return
	}
	if silences == nil {
		silences = []silence.Silence{}
	}
	recents, err := srv.silenceStore.GetRecentlyArchived()
	if err != nil {
		httputils.ReportError(w, err, "Failed to load recents.", http.StatusInternalServerError)
		return
	}
	silences = append(silences, recents...)
	if err := json.NewEncoder(w).Encode(silences); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) incidentHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ins, err := srv.incidentStore.GetAll()
	if err != nil {
		httputils.ReportError(w, err, "Failed to load incidents.", http.StatusInternalServerError)
		return
	}
	recents, err := srv.incidentStore.GetRecentlyResolved()
	if err != nil {
		httputils.ReportError(w, err, "Failed to load recents.", http.StatusInternalServerError)
		return
	}
	ins = append(ins, recents...)
	if err := json.NewEncoder(w).Encode(ins); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) recentIncidentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.FormValue("id")
	key := r.FormValue("key")
	ins, err := srv.incidentStore.GetRecentlyResolvedForID(id, key)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load incidents.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(ins); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) saveSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode silence creation request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "create-silence", req)
	silence, err := srv.silenceStore.Put(&req)
	if err != nil {
		httputils.ReportError(w, err, "Failed to create silence.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(silence); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) archiveSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode silence creation request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "archive-silence", req)
	silence, err := srv.silenceStore.Archive(req.Key)
	if err != nil {
		httputils.ReportError(w, err, "Failed to archive silence.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(silence); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) reactivateSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode silence reactivation request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "reactivate-silence", req)
	silence, err := srv.silenceStore.Reactivate(req.Key, req.Duration, srv.user(r))
	if err != nil {
		httputils.ReportError(w, err, "Failed to reactivate silence.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(silence); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) deleteSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var sil silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&sil); err != nil {
		httputils.ReportError(w, err, "Failed to decode silence deletion request.", http.StatusInternalServerError)
		return
	}
	auditlog.Log(r, "delete-silence", sil)
	if err := srv.silenceStore.Delete(sil.Key); err != nil {
		httputils.ReportError(w, err, "Failed to delete silence.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(sil); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

// newSilenceHandler creates and returns a new Silence pre-populated with good defaults.
func (srv *Server) newSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s := silence.New(srv.user(r))
	if err := json.NewEncoder(w).Encode(s); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

// See baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler).Methods("GET")

	// GETs
	r.HandleFunc("/_/incidents", srv.incidentHandler).Methods("GET")
	r.HandleFunc("/_/new_silence", srv.newSilenceHandler).Methods("GET")
	r.HandleFunc("/_/recent_incidents", srv.recentIncidentsHandler).Methods("GET")
	r.HandleFunc("/_/silences", srv.silencesHandler).Methods("GET")

	// POSTs
	r.HandleFunc("/_/add_note", srv.addNoteHandler).Methods("POST")
	r.HandleFunc("/_/add_silence_note", srv.addSilenceNoteHandler).Methods("POST")
	r.HandleFunc("/_/archive_silence", srv.archiveSilenceHandler).Methods("POST")
	r.HandleFunc("/_/assign", srv.assignHandler).Methods("POST")
	r.HandleFunc("/_/del_note", srv.delNoteHandler).Methods("POST")
	r.HandleFunc("/_/del_silence_note", srv.delSilenceNoteHandler).Methods("POST")
	r.HandleFunc("/_/del_silence", srv.deleteSilenceHandler).Methods("POST")
	r.HandleFunc("/_/reactivate_silence", srv.reactivateSilenceHandler).Methods("POST")
	r.HandleFunc("/_/save_silence", srv.saveSilenceHandler).Methods("POST")
	r.HandleFunc("/_/take", srv.takeHandler).Methods("POST")
	r.HandleFunc("/_/stats", srv.statsHandler).Methods("POST")
	r.HandleFunc("/_/incidents_in_range", srv.incidentsInRangeHandler).Methods("POST")
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	ret := []mux.MiddlewareFunc{}
	if !*baseapp.Local {
		ret = append(ret, login.ForceAuthMiddleware(login.DEFAULT_REDIRECT_URL), login.RestrictViewer)
	}
	return ret
}

func main() {
	baseapp.Serve(New, []string{"tree.skia.org"})
}
