// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/*
	Serves a webpage for easy management of Skia bugs.

	WARNING: This server is NOT secure and should not be made publicly
	accessible.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/securecookie"
	"skia.googlesource.com/buildbot.git/bug_chomper/go/issue_tracker"
	"skia.googlesource.com/buildbot.git/go/common"
)

const (
	CERT_FILE           = "certs/cert.pem"
	KEY_FILE            = "certs/key.pem"
	ISSUE_COMMENT       = "Edited by BugChomper"
	OAUTH_CALLBACK_PATH = "/oauth2callback"
	OAUTH_CONFIG_FILE   = "oauth_client_secret.json"
	LOCAL_HOST          = "127.0.0.1"
	MAX_SESSION_LEN     = time.Duration(3600 * time.Second)
	PRIORITY_PREFIX     = "Priority-"
	PROJECT_NAME        = "skia"
	COOKIE_NAME         = "BugChomperCookie"
)

// Flags:
var (
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	public         = flag.Bool("public", false, "Make this server publicly accessible.")
	graphiteServer = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
)

var (
	// templates is the list of html templates used by bug_chomper.
	templates *template.Template = nil

	scheme       = "http"
	hashKey      = securecookie.GenerateRandomKey(32)
	blockKey     = securecookie.GenerateRandomKey(32)
	secureCookie = securecookie.New(hashKey, blockKey)
)

func init() {
	// Change the current working directory to two directories up from this
	// source file so that we can read templates.
	_, filename, _, _ := runtime.Caller(0)
	cwd := filepath.Join(filepath.Dir(filename), "../..")
	if err := os.Chdir(cwd); err != nil {
		glog.Fatal(err)
	}

	templates = template.Must(template.ParseFiles(
		filepath.Join(cwd, "templates/bug_chomper.html"),
		filepath.Join(cwd, "templates/submitted.html"),
		filepath.Join(cwd, "templates/error.html"),
	))
}

// SessionState contains data for a given session.
type SessionState struct {
	IssueTracker   *issue_tracker.IssueTracker
	OrigRequestURL string
	SessionStart   time.Time
}

// getAbsoluteURL returns the absolute URL of the given Request.
func getAbsoluteURL(r *http.Request) string {
	return scheme + "://" + r.Host + r.URL.Path
}

// getOAuth2CallbackURL returns a callback URL to be used by the OAuth2 login
// page.
func getOAuth2CallbackURL(r *http.Request) string {
	return scheme + "://" + r.Host + OAUTH_CALLBACK_PATH
}

func saveSession(session *SessionState, w http.ResponseWriter, r *http.Request) error {
	encodedSession, err := secureCookie.Encode(COOKIE_NAME, session)
	if err != nil {
		return fmt.Errorf("unable to encode session state: %s", err)
	}
	cookie := &http.Cookie{
		Name:     COOKIE_NAME,
		Value:    encodedSession,
		Domain:   strings.Split(r.Host, ":")[0],
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	return nil
}

// makeSession creates a new session for the Request.
func makeSession(w http.ResponseWriter, r *http.Request) (*SessionState, error) {
	glog.Info("Creating new session.")
	// Create the session state.
	issueTracker, err := issue_tracker.MakeIssueTracker(
		OAUTH_CONFIG_FILE, getOAuth2CallbackURL(r))
	if err != nil {
		return nil, fmt.Errorf("unable to create IssueTracker for session: %s", err)
	}
	session := SessionState{
		IssueTracker:   issueTracker,
		OrigRequestURL: getAbsoluteURL(r),
		SessionStart:   time.Now(),
	}

	// Encode and store the session state.
	if err := saveSession(&session, w, r); err != nil {
		return nil, err
	}

	return &session, nil
}

// getSession retrieves the active SessionState or creates and returns a new
// SessionState.
func getSession(w http.ResponseWriter, r *http.Request) (*SessionState, error) {
	cookie, err := r.Cookie(COOKIE_NAME)
	if err != nil {
		glog.Info("No cookie found! Starting new session.")
		return makeSession(w, r)
	}
	var session SessionState
	if err := secureCookie.Decode(COOKIE_NAME, cookie.Value, &session); err != nil {
		glog.Infof("Invalid or corrupted session. Starting another: %s", err)
		return makeSession(w, r)
	}

	currentTime := time.Now()
	if currentTime.Sub(session.SessionStart) > MAX_SESSION_LEN {
		glog.Infof("Session starting at %s is expired. Starting another.",
			session.SessionStart.Format(time.RFC822))
		return makeSession(w, r)
	}
	saveSession(&session, w, r)
	return &session, nil
}

// reportError serves the error page with the given message.
func reportError(w http.ResponseWriter, msg string, code int) {
	errData := struct {
		Code       int
		CodeString string
		Message    string
	}{
		Code:       code,
		CodeString: http.StatusText(code),
		Message:    msg,
	}
	w.WriteHeader(code)
	err := templates.ExecuteTemplate(w, "error.html", errData)
	if err != nil {
		glog.Error("Failed to display error.html!!")
	}
}

// makeBugChomperPage builds and serves the BugChomper page.
func makeBugChomperPage(w http.ResponseWriter, r *http.Request) {
	session, err := getSession(w, r)
	if err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	issueTracker := session.IssueTracker
	user, err := issueTracker.GetLoggedInUser()
	if err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	glog.Info("Loading bugs for " + user)
	bugList, err := issueTracker.GetBugs(PROJECT_NAME, user)
	if err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bugsById := make(map[string]*issue_tracker.Issue)
	bugsByPriority := make(map[string][]*issue_tracker.Issue)
	for _, bug := range bugList.Items {
		bugsById[strconv.Itoa(bug.Id)] = bug
		var bugPriority string
		for _, label := range bug.Labels {
			if strings.HasPrefix(label, PRIORITY_PREFIX) {
				bugPriority = label[len(PRIORITY_PREFIX):]
			}
		}
		if _, ok := bugsByPriority[bugPriority]; !ok {
			bugsByPriority[bugPriority] = make(
				[]*issue_tracker.Issue, 0)
		}
		bugsByPriority[bugPriority] = append(
			bugsByPriority[bugPriority], bug)
	}
	bugsJson, err := json.Marshal(bugsById)
	if err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		Title          string
		User           string
		BugsJson       template.JS
		BugsByPriority *map[string][]*issue_tracker.Issue
		Priorities     []string
		PriorityPrefix string
	}{
		Title:          "BugChomper",
		User:           user,
		BugsJson:       template.JS(string(bugsJson)),
		BugsByPriority: &bugsByPriority,
		Priorities:     issue_tracker.BugPriorities,
		PriorityPrefix: PRIORITY_PREFIX,
	}

	if err := templates.ExecuteTemplate(w, "bug_chomper.html", data); err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// authIfNeeded determines whether the current user is logged in. If not, it
// redirects to a login page. Returns true if the user is redirected and false
// otherwise.
func authIfNeeded(w http.ResponseWriter, r *http.Request) bool {
	session, err := getSession(w, r)
	if err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	issueTracker := session.IssueTracker
	if !issueTracker.IsAuthenticated() {
		loginURL := issueTracker.MakeAuthRequestURL()
		glog.Info("Redirecting for login:", loginURL)
		http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)
		return true
	}
	return false
}

// submitData attempts to submit data from a POST request to the IssueTracker.
func submitData(w http.ResponseWriter, r *http.Request) {
	session, err := getSession(w, r)
	if err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	issueTracker := session.IssueTracker
	edits := r.FormValue("all_edits")
	var editsMap map[string]*issue_tracker.Issue
	if err := json.Unmarshal([]byte(edits), &editsMap); err != nil {
		errMsg := "Could not parse edits from form response: " + err.Error()
		reportError(w, errMsg, http.StatusInternalServerError)
		return
	}
	data := struct {
		Title    string
		Message  string
		BackLink string
	}{}
	if len(editsMap) == 0 {
		data.Title = "No Changes Submitted"
		data.Message = "You didn't change anything!"
		data.BackLink = ""
		if err := templates.ExecuteTemplate(w, "submitted.html", data); err != nil {
			reportError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
	errorList := make([]error, 0)
	for issueId, newIssue := range editsMap {
		glog.Info("Editing issue " + issueId)
		if err := issueTracker.SubmitIssueChanges(newIssue, ISSUE_COMMENT); err != nil {
			errorList = append(errorList, err)
		}
	}
	if len(errorList) > 0 {
		errorStrings := ""
		for _, err := range errorList {
			errorStrings += err.Error() + "\n"
		}
		errMsg := "Not all changes could be submitted: \n" + errorStrings
		reportError(w, errMsg, http.StatusInternalServerError)
		return
	}
	data.Title = "Submitted Changes"
	data.Message = "Your changes were submitted to the issue tracker."
	data.BackLink = ""
	if err := templates.ExecuteTemplate(w, "submitted.html", data); err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	return
}

// handleBugChomper handles HTTP requests for the bug_chomper page.
func handleBugChomper(w http.ResponseWriter, r *http.Request) {
	if authIfNeeded(w, r) {
		return
	}
	switch r.Method {
	case "GET":
		makeBugChomperPage(w, r)
	case "POST":
		submitData(w, r)
	}
}

// handleOAuth2Callback handles callbacks from the OAuth2 sign-in.
func handleOAuth2Callback(w http.ResponseWriter, r *http.Request) {
	session, err := getSession(w, r)
	if err != nil {
		reportError(w, err.Error(), http.StatusInternalServerError)
	}
	issueTracker := session.IssueTracker
	invalidLogin := "Invalid login credentials"
	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		reportError(w, invalidLogin+": "+err.Error(), http.StatusForbidden)
		return
	}
	code, ok := params["code"]
	if !ok {
		reportError(w, invalidLogin+": redirect did not include auth code.",
			http.StatusForbidden)
		return
	}
	glog.Info("Upgrading auth token:", code[0])
	if err := issueTracker.UpgradeCode(code[0]); err != nil {
		errMsg := "failed to upgrade token: " + err.Error()
		reportError(w, errMsg, http.StatusForbidden)
		return
	}
	if err := saveSession(session, w, r); err != nil {
		reportError(w, "failed to save session: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, session.OrigRequestURL, http.StatusTemporaryRedirect)
	return
}

// handleRoot is the handler function for all HTTP requests at the root level.
func handleRoot(w http.ResponseWriter, r *http.Request) {
	glog.Info("Fetching " + r.URL.Path)
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		handleBugChomper(w, r)
		return
	}
	http.NotFound(w, r)
}

// Run the BugChomper server.
func main() {
	common.InitWithMetrics("bug_chomper", *graphiteServer)

	http.HandleFunc("/", handleRoot)
	http.HandleFunc(OAUTH_CALLBACK_PATH, handleOAuth2Callback)
	http.Handle("/res/", http.FileServer(http.Dir("./")))
	glog.Info("Server is running at " + scheme + "://" + LOCAL_HOST + *port)
	var err error
	if *public {
		glog.Warning("WARNING: This server is not secure and should not be made " +
			"publicly accessible.")
		scheme = "https"
		err = http.ListenAndServeTLS(*port, CERT_FILE, KEY_FILE, nil)
	} else {
		scheme = "http"
		err = http.ListenAndServe(LOCAL_HOST+*port, nil)
	}
	if err != nil {
		glog.Error(err)
	}
}
