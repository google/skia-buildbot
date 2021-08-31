package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/tree_status/go/types"
)

const (
	STATUS_DS_KIND = "Status"
)

var (
	// Mutex that guards changes to the status datastore.
	statusMtx sync.RWMutex
)

// getRepoNamespace returns which DB namespace to use for the specified repo.
// All repos except the main skia repo uses "tree-status-${repo}" namespace.
// The main skia repo uses "tree-status" (without the repo specified) for
// backwards compatibility.
func getRepoNamespace(repo string) string {
	repoNamespace := *namespace
	if repo != defaultSkiaRepo {
		repoNamespace = fmt.Sprintf("%s_%s", *namespace, repo)
	}
	return repoNamespace
}

func AddStatus(repo, message, username, generalState, rollers string) error {
	s := &types.Status{
		Date:         time.Now(),
		Message:      message,
		Rollers:      rollers,
		Username:     username,
		GeneralState: generalState,
	}

	key := &datastore.Key{
		Kind:      STATUS_DS_KIND,
		Namespace: getRepoNamespace(repo),
	}
	if _, err := dsClient.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		var err error
		if _, err = tx.Put(key, s); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("Failed to add status: %s", err)
	}
	return nil
}

func GetLatestStatus(repo string) (*types.Status, error) {
	statuses, err := GetStatuses(repo, 1)
	if err != nil {
		return nil, err
	}
	return statuses[0], nil
}

func GetStatuses(repo string, num int) ([]*types.Status, error) {
	statuses := []*types.Status{}
	q := datastore.NewQuery("Status").Namespace(getRepoNamespace(repo)).Order("-date").Limit(num)
	it := dsClient.Run(context.TODO(), q)
	for {
		s := &types.Status{}
		_, err := it.Next(s)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of statuses: %s", err)
		}
		statuses = append(statuses, s)
	}
	return statuses, nil
}

// HTTP Handlers

func (srv *Server) treeStateDefaultRepoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	repo := mux.Vars(r)["repo"]
	if !IsRepoSupported(repo) {
		// Use the default repo if it is specified else throw an error.
		if srv.skiaRepoSpecified {
			repo = defaultSkiaRepo
		} else {
			httputils.ReportError(w, nil, fmt.Sprintf("The repo %s is not supported", repo), http.StatusBadRequest)
			return
		}
	}

	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
		"Repo":  repo,
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) bannerStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Setting allow-origin and allow-credentials to make it possible to access
	// the private uberproxy URL from javascript using fetch from status-internal
	// and from Gerrit plugins. This is safe to do because:
	// * For the public instance this endpoint is available without
	//   authentication anyway.
	// * The private instance is protected via uberproxy so only Googlers
	//   will be able to access it.
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	repo := mux.Vars(r)["repo"]
	if !IsRepoSupported(repo) {
		// Use the default repo if it is specified else throw an error.
		if srv.skiaRepoSpecified {
			repo = defaultSkiaRepo
		} else {
			httputils.ReportError(w, nil, fmt.Sprintf("The repo %s is not supported", repo), http.StatusBadRequest)
			return
		}
	}

	statusMtx.RLock()
	defer statusMtx.RUnlock()

	statuses, err := GetStatuses(repo, 1)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for recent statuses.", http.StatusInternalServerError)
		return
	}
	var status interface{}
	if len(statuses) == 0 {
		status = map[string]string{}
	} else {
		// This is the weird python date format expected by the CQ. Eg: 2020-02-25 14:47:26.253187.
		d := statuses[0].Date
		expectedDateFormat := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d.%06d", d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), d.Second(), d.Nanosecond()/1000)
		status = struct {
			Username     string `json:"username"`
			Date         string `json:"date"`
			Message      string `json:"message"`
			GeneralState string `json:"general_state"`
		}{
			Username:     statuses[0].Username,
			Date:         expectedDateFormat,
			Message:      statuses[0].Message,
			GeneralState: statuses[0].GeneralState,
		}
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) recentStatusesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	repo := mux.Vars(r)["repo"]
	if !IsRepoSupported(repo) {
		httputils.ReportError(w, nil, fmt.Sprintf("The repo %s is not supported", repo), http.StatusBadRequest)
		return
	}

	statusMtx.RLock()
	defer statusMtx.RUnlock()

	statuses, err := GetStatuses(repo, 25)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for recent statuses.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) addStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	repo := mux.Vars(r)["repo"]
	if !IsRepoSupported(repo) {
		httputils.ReportError(w, nil, fmt.Sprintf("The repo %s is not supported", repo), http.StatusBadRequest)
		return
	}

	user := srv.user(r)
	if !srv.modify.Member(user) {
		httputils.ReportError(w, nil, "You do not have access to set the tree status.", http.StatusInternalServerError)
		return
	}

	// Parse the request.
	m := struct {
		Message string `json:"message"`
		Rollers string `json:"rollers"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		httputils.ReportError(w, err, "Failed to decode request.", http.StatusInternalServerError)
		return
	}
	message := m.Message
	rollers := m.Rollers

	// Validate the message.
	containsOpenState := strings.Contains(strings.ToLower(message), types.OpenState)
	containsCautionState := strings.Contains(strings.ToLower(message), types.CautionState)
	containsClosedState := strings.Contains(strings.ToLower(message), types.ClosedState)
	if (containsOpenState && containsCautionState) ||
		(containsCautionState && containsClosedState) ||
		(containsClosedState && containsOpenState) {
		httputils.ReportError(w, nil, fmt.Sprintf("Cannot specify two keywords from (%s, %s, %s) in a status message.", types.OpenState, types.CautionState, types.ClosedState), http.StatusBadRequest)
		return
	} else if !(containsOpenState || containsCautionState || containsClosedState) {
		httputils.ReportError(w, nil, fmt.Sprintf("Must specify either (%s, %s, %s) somewhere in the status message.", types.OpenState, types.CautionState, types.ClosedState), http.StatusBadRequest)
		return
	} else if containsOpenState && rollers != "" {
		httputils.ReportError(w, nil, fmt.Sprintf("Waiting for rollers should only be used with %s or %s states", types.CautionState, types.ClosedState), http.StatusBadRequest)
		return
	}

	// Figure out the state.
	var generalState string
	if containsClosedState {
		generalState = types.ClosedState
	} else if containsCautionState {
		generalState = types.CautionState
	} else {
		generalState = types.OpenState
	}

	statusMtx.Lock()
	defer statusMtx.Unlock()

	// Stop watching any previously defined autorollers.
	StopWatchingAutorollers()
	// Add status to datastore.
	if err := AddStatus(repo, message, user, generalState, rollers); err != nil {
		httputils.ReportError(w, err, "Failed to add message to the datastore", http.StatusInternalServerError)
		return
	}
	// Start watching any newly defined autorollers.
	StartWatchingAutorollers(rollers)

	// Return updated list of the most recent tree statuses.
	statuses, err := GetStatuses(repo, 25)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for recent statuses.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
		return
	}
}
