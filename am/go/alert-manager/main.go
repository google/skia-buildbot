package main

import (
	"context"
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
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/am/go/audit"
	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/am/go/reminder"
	"go.skia.org/infra/am/go/silence"
	"go.skia.org/infra/am/go/types"
	"go.skia.org/infra/email/go/emailclient"
	"go.skia.org/infra/go/alerts"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	assignGroup        = flag.String("assign_group", "google/skia-root@google.com", "The chrome infra auth group to use for users incidents can be assigned to.")
	authGroup          = flag.String("auth_group", "google/skia-staff@google.com", "The chrome infra auth group to use for restricting access.")
	chromeInfraAuthJWT = flag.String("chrome_infra_auth_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	host               = flag.String("host", "am.skia.org", "HTTP service host")
	namespace          = flag.String("namespace", "", "The Cloud Datastore namespace, such as 'alert-manager'.")
	internalPort       = flag.String("internal_port", ":9000", "HTTP internal service address (e.g., ':9000') for unauthenticated in-cluster requests.")
	project            = flag.String("project", "skia-public", "The Google Cloud project name.")

	silenceRecentlyExpiredDuration = flag.Duration("recently_expired_duration", 2*time.Hour, "Incidents with silences that recently expired within this duration are shown with an icon.")
)

const (
	// expireDuration is the time to wait before expiring an incident.
	expireDuration = 5 * time.Minute

	// Constants for sending reminder emails.
	reminderNumThreshold       = 10
	reminderDurationThreshold  = 600
	reminderDurationPercentage = 0.60
)

// server is the state of the server.
type server struct {
	incidentStore *incident.Store
	silenceStore  *silence.Store
	templates     *template.Template
	allow         allowed.Allow // Who is allowed to use the site.
	assign        allowed.Allow // A list of people that incidents can be assigned to.
}

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	var allow allowed.Allow
	var assign allowed.Allow
	if !*baseapp.Local {
		ts, err := auth.NewJWTServiceAccountTokenSource("", *chromeInfraAuthJWT, auth.ScopeUserinfoEmail)
		if err != nil {
			return nil, err
		}
		client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		allow, err = allowed.NewAllowedFromChromeInfraAuth(client, *authGroup)
		if err != nil {
			return nil, err
		}
		assign, err = allowed.NewAllowedFromChromeInfraAuth(client, *assignGroup)
		if err != nil {
			return nil, err
		}
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
		assign = allowed.NewAllowedFromList([]string{"betty@example.org", "fred@example.org", "barney@example.org", "wilma@example.org"})
	}

	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, pubsub.ScopePubSub, "https://www.googleapis.com/auth/datastore")
	if err != nil {
		return nil, err
	}

	if *namespace == "" {
		return nil, fmt.Errorf("The --namespace flag is required. See infra/DATASTORE.md for format details.\n")
	}
	if !*baseapp.Local && !util.In(*namespace, []string{ds.ALERT_MANAGER_NS}) {
		return nil, fmt.Errorf("When running in prod the datastore namespace must be a known value.")
	}
	if err := ds.InitWithOpt(*project, *namespace, option.WithTokenSource(ts)); err != nil {
		return nil, fmt.Errorf("Failed to init Cloud Datastore: %s", err)
	}

	sub, err := sub.New(ctx, *baseapp.Local, *project, alerts.TOPIC, 1)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create subscription.")
	}

	srv := &server{
		incidentStore: incident.NewStore(ds.DS, []string{"kubernetes_pod_name", "instance", "pod_template_hash"}),
		silenceStore:  silence.NewStore(ds.DS),
		allow:         allow,
		assign:        assign,
	}
	srv.loadTemplates()

	// Start goroutine to send reminders to active alert owners.
	reminder.StartReminderTicker(srv.incidentStore, srv.silenceStore, emailclient.New())

	// livenesses gets populated as notifications arrive.
	livenesses := map[string]metrics2.Liveness{}

	// Process all incoming PubSub requests.
	go func() {
		for {
			err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				msg.Ack()
				var m map[string]string
				if err := json.Unmarshal(msg.Data, &m); err != nil {
					sklog.Error(err)
					return
				}
				if m[alerts.TYPE] == alerts.TYPE_HEALTHZ {
					location := m[alerts.LOCATION]
					sklog.Infof("healthz received: %q", location)
					if l, ok := livenesses[m[alerts.LOCATION]]; ok {
						l.Reset()
					} else {
						livenesses[location] = metrics2.NewLiveness("alert_to_pubsub_alive", map[string]string{alerts.LOCATION: location})
					}
				} else {
					if _, err := srv.incidentStore.AlertArrival(m); err != nil {
						sklog.Errorf("Error processing alert: %s", err)
					}
				}
			})
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()

	// This is really just a backstop in case we miss a resolved event for the incident.
	go func() {
		for range time.Tick(1 * time.Minute) {
			ins, err := srv.incidentStore.GetAll()
			if err != nil {
				sklog.Errorf("Failed to load incidents: %s", err)
				continue
			}
			now := time.Now()
			for _, in := range ins {
				// If it was last updated too long ago then it should be archived.
				if time.Unix(in.LastSeen, 0).Add(expireDuration).Before(now) {
					if _, err := srv.incidentStore.Archive(in.Key); err != nil {
						sklog.Errorf("Failed to archive incident: %s", err)
					}
				}
			}
		}
	}()

	srv.startInternalServer()

	return srv, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
	))
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

type addNoteRequest struct {
	Text string `json:"text"`
	Key  string `json:"key"`
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

func (srv *server) addNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req addNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request.", http.StatusInternalServerError)
		return
	}
	audit.Log(r, "add-note", req)

	note := note.Note{
		Text:   req.Text,
		TS:     time.Now().Unix(),
		Author: srv.user(r),
	}
	in, err := srv.incidentStore.AddNote(req.Key, note)
	if err != nil {
		httputils.ReportError(w, err, "Failed to add incident note.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) addSilenceNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req addNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request.", http.StatusInternalServerError)
		return
	}
	audit.Log(r, "add-silence-note", req)

	note := note.Note{
		Text:   req.Text,
		TS:     time.Now().Unix(),
		Author: srv.user(r),
	}
	in, err := srv.silenceStore.AddNote(req.Key, note)
	if err != nil {
		httputils.ReportError(w, err, "Failed to add silence note.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type delNoteRequest struct {
	Index int    `json:"index"`
	Key   string `json:"key"`
}

func (srv *server) delNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req delNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request.", http.StatusInternalServerError)
		return
	}
	audit.Log(r, "del-note", req)
	in, err := srv.incidentStore.DeleteNote(req.Key, req.Index)
	if err != nil {
		httputils.ReportError(w, err, "Failed to del incident note.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) delSilenceNoteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req delNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request.", http.StatusInternalServerError)
		return
	}
	audit.Log(r, "del-silence-note", req)
	in, err := srv.silenceStore.DeleteNote(req.Key, req.Index)
	if err != nil {
		httputils.ReportError(w, err, "Failed to del silence note.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type TakeRequest struct {
	Key string `json:"key"`
}

func (srv *server) takeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req TakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode take request.", http.StatusInternalServerError)
		return
	}
	audit.Log(r, "take", req)

	in, err := srv.incidentStore.Assign(req.Key, srv.user(r))
	if err != nil {
		httputils.ReportError(w, err, "Failed to assign.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type StatsResponseSlice types.StatsResponse

func (p StatsResponseSlice) Len() int           { return len(p) }
func (p StatsResponseSlice) Less(i, j int) bool { return p[i].Num > p[j].Num }
func (p StatsResponseSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (srv *server) statsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req types.StatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode stats request.", http.StatusInternalServerError)
		return
	}
	ins, err := srv.incidentStore.GetRecentlyResolvedInRange(req.Range)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for Incidents.", http.StatusInternalServerError)
	}
	count := map[string]*types.Stat{}
	for _, in := range ins {
		if stat, ok := count[in.ID]; !ok {
			count[in.ID] = &types.Stat{
				Num:      1,
				Incident: in,
			}
		} else {
			stat.Num += 1
		}
	}
	ret := types.StatsResponse{}
	for _, v := range count {
		ret = append(ret, v)
	}
	sort.Sort(StatsResponseSlice(ret))
	if err := json.NewEncoder(w).Encode(ret); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) incidentsInRangeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req types.IncidentsInRangeRequest
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

func (srv *server) assignHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode take request.", http.StatusInternalServerError)
		return
	}
	audit.Log(r, "assign", req)
	in, err := srv.incidentStore.Assign(req.Key, req.Email)
	if err != nil {
		httputils.ReportError(w, err, "Failed to assign.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(in); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

type AssignMultipleRequest struct {
	Keys  []string `json:"keys"`
	Email string   `json:"email"`
}

func (srv *server) assignMultipleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req AssignMultipleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode assign multiple request.", http.StatusInternalServerError)
		return
	}
	audit.Log(r, "assign multiple", req)

	for _, k := range req.Keys {
		if _, err := srv.incidentStore.Assign(k, req.Email); err != nil {
			httputils.ReportError(w, err, "Failed to assign multiple.", http.StatusInternalServerError)
			return
		}
	}

	ins, err := srv.getActiveAndRecentlyResolvedIncidents()
	if err != nil {
		httputils.ReportError(w, err, "Failed to load incidents.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(ins); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) getActiveAndRecentlyResolvedIncidents() ([]incident.Incident, error) {
	ins, err := srv.incidentStore.GetAll()
	if err != nil {
		return nil, fmt.Errorf("Failed to load incidents: %s", err)
	}
	recents, err := srv.incidentStore.GetRecentlyResolved()
	if err != nil {
		return nil, fmt.Errorf("Failed to load recents: %s", err)
	}
	ins = append(ins, recents...)
	return ins, nil
}

func (srv *server) emailsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	emails := srv.assign.Emails()
	sort.Strings(emails)
	if err := json.NewEncoder(w).Encode(&emails); err != nil {
		sklog.Errorf("Failed to encode emails: %s", err)
	}
}

func (srv *server) silencesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	silences, err := srv.silenceStore.GetAll()
	if err != nil {
		httputils.ReportError(w, err, "Failed to load recents.", http.StatusInternalServerError)
		return
	}
	if silences == nil {
		silences = []silence.Silence{}
	}
	recents, err := srv.silenceStore.GetRecentlyArchived(0)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load recents.", http.StatusInternalServerError)
		return
	}
	silences = append(silences, recents...)
	if err := json.NewEncoder(w).Encode(silences); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) auditLogsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logs, err := audit.GetLogs(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to load audit logs.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) incidentHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ins, err := srv.getActiveAndRecentlyResolvedIncidents()
	if err != nil {
		httputils.ReportError(w, err, "Failed to load incidents.", http.StatusInternalServerError)
		return
	}

	idsToRecentlyExpiredSilences := map[string]bool{}
	archivedSilences, err := srv.silenceStore.GetRecentlyArchived(*silenceRecentlyExpiredDuration)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load archived silences.", http.StatusInternalServerError)
	}
	if len(archivedSilences) > 0 {
		for _, i := range ins {
			idsToRecentlyExpiredSilences[i.ID] = i.IsSilenced(archivedSilences, false)
		}
	}
	resp := types.IncidentsResponse{
		Incidents:                    ins,
		IdsToRecentlyExpiredSilences: idsToRecentlyExpiredSilences,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) recentIncidentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.FormValue("id")
	key := r.FormValue("key")
	ins, err := srv.incidentStore.GetRecentlyResolvedForID(id, key)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load incidents.", http.StatusInternalServerError)
		return
	}

	// Get recently archived silences to see if the incident recently expired.
	archivedSilences, err := srv.silenceStore.GetRecentlyArchived(*silenceRecentlyExpiredDuration)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load archived silences.", http.StatusInternalServerError)
		return
	}
	recentlyExpired := false
	if len(ins) > 0 {
		recentlyExpired = ins[0].IsSilenced(archivedSilences, false)
	}

	resp := types.RecentIncidentsResponse{
		Incidents:              ins,
		Flaky:                  incident.AreIncidentsFlaky(ins, reminderNumThreshold, reminderDurationThreshold, reminderDurationPercentage),
		RecentlyExpiredSilence: recentlyExpired,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) saveSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode silence creation request.", http.StatusInternalServerError)
		return
	}
	if err := req.ValidateRegexes(); err != nil {
		httputils.ReportError(w, err, "Silence has invalid regex.", http.StatusInternalServerError)
		return
	}

	audit.Log(r, "create-silence", req)
	silence, err := srv.silenceStore.Put(&req)
	if err != nil {
		httputils.ReportError(w, err, "Failed to create silence.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(silence); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) archiveSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode silence creation request.", http.StatusInternalServerError)
		return
	}
	if err := req.ValidateRegexes(); err != nil {
		httputils.ReportError(w, err, "Silence has invalid regex.", http.StatusInternalServerError)
		return
	}

	audit.Log(r, "archive-silence", req)
	silence, err := srv.silenceStore.Archive(req.Key)
	if err != nil {
		httputils.ReportError(w, err, "Failed to archive silence.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(silence); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) reactivateSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode silence reactivation request.", http.StatusInternalServerError)
		return
	}
	if err := req.ValidateRegexes(); err != nil {
		httputils.ReportError(w, err, "Silence has invalid regex.", http.StatusInternalServerError)
		return
	}

	audit.Log(r, "reactivate-silence", req)
	silence, err := srv.silenceStore.Reactivate(req.Key, req.Duration, srv.user(r))
	if err != nil {
		httputils.ReportError(w, err, "Failed to reactivate silence.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(silence); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *server) deleteSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var sil silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&sil); err != nil {
		httputils.ReportError(w, err, "Failed to decode silence deletion request.", http.StatusInternalServerError)
		return
	}
	audit.Log(r, "delete-silence", sil)
	if err := srv.silenceStore.Delete(sil.Key); err != nil {
		httputils.ReportError(w, err, "Failed to delete silence.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(sil); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

// newSilenceHandler creates and returns a new Silence pre-populated with good defaults.
func (srv *server) newSilenceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s := silence.New(srv.user(r))
	if err := json.NewEncoder(w).Encode(s); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler).Methods("GET")

	// GETs
	r.HandleFunc("/_/emails", srv.emailsHandler).Methods("GET")
	r.HandleFunc("/_/incidents", srv.incidentHandler).Methods("GET")
	r.HandleFunc("/_/new_silence", srv.newSilenceHandler).Methods("GET")
	r.HandleFunc("/_/recent_incidents", srv.recentIncidentsHandler).Methods("GET")
	r.HandleFunc("/_/silences", srv.silencesHandler).Methods("GET")

	// POSTs
	r.HandleFunc("/_/add_note", srv.addNoteHandler).Methods("POST")
	r.HandleFunc("/_/add_silence_note", srv.addSilenceNoteHandler).Methods("POST")
	r.HandleFunc("/_/archive_silence", srv.archiveSilenceHandler).Methods("POST")
	r.HandleFunc("/_/assign", srv.assignHandler).Methods("POST")
	r.HandleFunc("/_/assign_multiple", srv.assignMultipleHandler).Methods("POST")
	r.HandleFunc("/_/audit_logs", srv.auditLogsHandler).Methods("POST")
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
func (srv *server) AddMiddleware() []mux.MiddlewareFunc {
	ret := []mux.MiddlewareFunc{}
	if !*baseapp.Local {
		ret = append(ret, login.ForceAuthMiddleware(login.DEFAULT_REDIRECT_URL), login.RestrictViewer)
	}
	return ret
}

func (srv *server) startInternalServer() {
	// Internal endpoints that are only accessible from within the cluster.
	unprotected := mux.NewRouter()
	unprotected.HandleFunc("/_/incidents", srv.incidentHandler).Methods("GET")
	unprotected.HandleFunc("/_/silences", srv.silencesHandler).Methods("GET")
	go func() {
		sklog.Fatal(http.ListenAndServe(*internalPort, unprotected))
	}()
}

func main() {
	// Parse flags to be able to send *host to baseapp.Serve
	flag.Parse()
	baseapp.Serve(New, []string{*host})
}
