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
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/bug_chomper/go/issue_tracker"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
)

const (
	ISSUE_COMMENT       = "Edited by BugChomper"
	OAUTH_CALLBACK_PATH = "/oauth2callback/"
	OAUTH_CONFIG_FILE   = "oauth_client_secret.json"
	PRIORITY_PREFIX     = "Priority-"
	PROJECT_NAME        = "skia"
)

// Flags:
var (
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	useMetadata    = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	testing        = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	graphiteServer = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

var (
	// templates is the list of html templates used by bug_chomper.
	templates *template.Template = nil
)

func loadTemplates() {
	// Change the current working directory to two directories up from this
	// source file so that we can read templates.
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}

	templates = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/bug_chomper.html"),
		filepath.Join(*resourcesDir, "templates/submitted.html"),
		filepath.Join(*resourcesDir, "templates/error.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
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
	// Redirect for login if needed.
	user := login.LoggedInAs(r)
	if user == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
		return
	}
	glog.Infof("Logged in as %s", user)

	issueTracker := issue_tracker.New(login.GetHttpClient(r))
	w.Header().Set("Content-Type", "text/html")
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

// submitData attempts to submit data from a POST request to the IssueTracker.
func submitData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	issueTracker := issue_tracker.New(login.GetHttpClient(r))
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

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", makeBugChomperPage).Methods("GET")
	r.HandleFunc("/", submitData).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc(OAUTH_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	glog.Info("Server is running at " + serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

// Run the BugChomper server.
func main() {
	common.InitWithMetrics("bug_chomper", graphiteServer)

	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	loadTemplates()

	if *testing {
		*useMetadata = false
	}
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}

	// By default use a set of credentials setup for localhost access.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = serverURL + "/oauth2callback/"
	if *useMetadata {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, strings.Join(issue_tracker.OAUTH_SCOPE, " "), login.DEFAULT_DOMAIN_WHITELIST, false)

	runServer(serverURL)
}
