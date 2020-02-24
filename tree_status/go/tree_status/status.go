package main

// TODO(rmistry): Need to move this whole package maybe to use firestore.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

const (
	STATUS_DS_KIND = "Status"
)

//var (
//	// DS is the Cloud Datastore client. Valid after DatastoreInit() has been called.
//	DS *datastore.Client
//)

// Status - A Tree status update.
type Status struct {
	// Key      string    `json:"key" datastore:"key"` // Key is the web-safe serialized Datastore key for the Tree Status.
	Date     time.Time `json:"date" datastore:"date"`
	Message  string    `json:"message" datastore:"message"`
	Rollers  string    `json:"rollers" datastore:"rollers"`
	Username string    `json:"username" datastore:"username"`
	// Only specified for backwards compatibility.
	FirstRev int `json:"first_rev,omitempty" datastore:"first_rev"`
	LastRev  int `json:"last_rev,omitempty" datastore:"last_rev"`
}

func AddStatus(message, username, rollers string) error {
	s := &Status{
		Date:     time.Now(),
		Message:  message,
		Rollers:  rollers,
		Username: username,
	}
	s.FirstRev = 1
	s.LastRev = 1

	key := &datastore.Key{
		Kind:      STATUS_DS_KIND,
		Namespace: "",
	}
	_, err := DS.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		var err error
		if _, err = tx.Put(key, s); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Failed to add status: %s", err)
	}
	return nil
}

func GetLatestStatus() (*Status, error) {
	statuses, err := GetStatuses(1)
	if err != nil {
		return nil, err
	}
	return statuses[0], nil
}

func GetStatuses(num int) ([]*Status, error) {
	statuses := []*Status{}
	q := datastore.NewQuery("Status").Namespace("").Order("-date").Limit(num)
	it := DS.Run(context.TODO(), q)
	for {
		s := &Status{}
		// Combine with below
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

//func StatusInit(ts oauth2.TokenSource, project string, ns string, local bool) error {
//	var err error
//	DS, err = datastore.NewClient(context.Background(), project, option.WithTokenSource(ts))
//	if err != nil {
//		return skerr.Wrapf(err, "Failed to initialize Cloud Datastore for tree status")
//	}
//	return err
//}

// HTTP Handlers

func (srv *Server) recentStatusesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	statuses, err := GetStatuses(25)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for recent statuses.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) getAutorollersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	as, err := GetAutorollStatuses(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get autoroll statuses.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(as); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) addStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	user := srv.user(r)
	if !srv.access.Member(user) {
		httputils.ReportError(w, nil, "You do not have access to set the tree status.", http.StatusInternalServerError)
		return
	}

	// Get the message from the request.
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
	fmt.Println("XXXXXXXXXXXXXXXXXXXXXXXXXx")
	fmt.Printf("%+v", m)

	// Validate the message. Extract into a function.
	containsOpenState := strings.Contains(strings.ToLower(message), OPEN_STATE)
	containsCautionState := strings.Contains(strings.ToLower(message), CAUTION_STATE)
	containsClosedState := strings.Contains(strings.ToLower(message), CLOSED_STATE)
	fmt.Println(containsOpenState)
	fmt.Println(containsCautionState)
	fmt.Println(containsClosedState)
	if (containsOpenState && containsCautionState) ||
		(containsCautionState && containsClosedState) ||
		(containsClosedState && containsOpenState) {
		httputils.ReportError(w, nil, fmt.Sprintf("Cannot specify two keywords from (%s, %s, %s) in a status message.", OPEN_STATE, CAUTION_STATE, CLOSED_STATE), http.StatusBadRequest)
		return
	} else if !(containsOpenState || containsCautionState || containsClosedState) {
		httputils.ReportError(w, nil, fmt.Sprintf("Must specify either (%s, %s, %s) somewhere in the status message.", OPEN_STATE, CAUTION_STATE, CLOSED_STATE), http.StatusBadRequest)
		return
	} else if containsOpenState && rollers != "" {
		httputils.ReportError(w, nil, fmt.Sprintf("Waiting for rollers should only be used with %s or %s states", CAUTION_STATE, CLOSED_STATE), http.StatusBadRequest)
		return
	}

	// ADD A MUTEX HERE!!!!!!!!!!!!!

	StopWatchingAutorollers()

	if err := AddStatus(message, user, rollers); err != nil {
		httputils.ReportError(w, err, "Failed to add message to the datastore", http.StatusInternalServerError)
		return
	}

	// Start the autorollers goroutine.
	StartWatchingAutorollers(rollers)

	// Return updated list of the most recent tree statuses.
	statuses, err := GetStatuses(25)
	if err != nil {
		httputils.ReportError(w, err, "Failed to query for recent statuses.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
		return
	}
}
