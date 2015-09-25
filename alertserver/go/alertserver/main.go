/*
	Provides alerting for Skia.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"
)

import (
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
)

import (
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/alertserver/go/rules"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
)

const (
	GMAIL_TOKEN_CACHE_FILE = "google_email_token.data"
	PARAM_INCLUDE_CATEGORY = "category"
	PARAM_EXCLUDE_CATEGORY = "excludeCategory"
)

var (
	alertManager *alerting.AlertManager = nil
	rulesList    []*rules.Rule          = nil

	alertsTemplate *template.Template = nil
	rulesTemplate  *template.Template = nil
)

// flags
var (
	graphiteServer        = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host                  = flag.String("host", "localhost", "HTTP service host")
	port                  = flag.String("port", ":8001", "HTTP service port (e.g., ':8001')")
	useMetadata           = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	emailClientIdFlag     = flag.String("email_clientid", "", "OAuth Client ID for sending email.")
	emailClientSecretFlag = flag.String("email_clientsecret", "", "OAuth Client Secret for sending email.")
	alertPollInterval     = flag.String("alert_poll_interval", "1s", "How often to check for new alerts.")
	alertsFile            = flag.String("alerts_file", "alerts.cfg", "Config file containing alert rules.")
	testing               = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	validateAndExit       = flag.Bool("validate_and_exit", false, "If set, just validate the config file and then exit.")
	workdir               = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir          = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

// StringIsInteresting returns true iff the string contains non-whitespace characters.
func StringIsInteresting(s string) bool {
	for _, c := range s {
		if !unicode.IsSpace(c) {
			return true
		}
	}
	return false
}

func reloadTemplates() {
	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.

	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	alertsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/alerts.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	rulesTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/rules.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func Init() {
	reloadTemplates()
}

func userHasEditRights(email string) bool {
	if strings.HasSuffix(email, "@google.com") {
		return true
	}
	return false
}

func getIntParam(name string, r *http.Request) (*int, error) {
	raw, ok := r.URL.Query()[name]
	if !ok {
		return nil, nil
	}
	v64, err := strconv.ParseInt(raw[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Invalid value for parameter %q: %s -- %v", name, raw, err)
	}
	v32 := int(v64)
	return &v32, nil
}

func makeAlertFilter(r *http.Request) func(*alerting.Alert) bool {
	includeCategories := []string{}
	excludeCategories := []string{}
	queryInclude, ok := r.URL.Query()[PARAM_INCLUDE_CATEGORY]
	if ok {
		includeCategories = queryInclude
	}
	queryExclude, ok := r.URL.Query()[PARAM_EXCLUDE_CATEGORY]
	if ok {
		excludeCategories = queryExclude
	}
	return func(a *alerting.Alert) bool {
		if len(includeCategories) > 0 {
			for _, include := range includeCategories {
				if a.Category == include {
					return true
				}
			}
			return false
		}
		for _, exclude := range excludeCategories {
			if a.Category == exclude {
				return false
			}
		}
		return true
	}
}

func alertJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := alertManager.WriteActiveAlertsJson(w, makeAlertFilter(r)); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

func handleAlert(alertId int64, comment string, until int, w http.ResponseWriter, r *http.Request) {
	email := login.LoggedInAs(r)
	if !userHasEditRights(email) {
		util.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "You must be logged in to an account with edit rights to do that.")
		return
	}

	action, ok := mux.Vars(r)["action"]
	if !ok {
		util.ReportError(w, r, fmt.Errorf("No action provided."), "No action provided.")
		return
	}

	if action == "dismiss" {
		glog.Infof("%s %d", action, alertId)
		if err := alertManager.Dismiss(alertId, email, comment); err != nil {
			util.ReportError(w, r, err, "Failed to dismiss alert.")
			return
		}
		return
	} else if action == "snooze" {
		if until == 0 {
			util.ReportError(w, r, fmt.Errorf("Invalid snooze time."), fmt.Sprintf("Invalid snooze time"))
			return
		}
		until := time.Unix(int64(until), 0)
		glog.Infof("%s %d until %v", action, alertId, until.String())
		if err := alertManager.Snooze(alertId, until, email, comment); err != nil {
			util.ReportError(w, r, err, "Failed to snooze alert.")
			return
		}
		return
	} else if action == "unsnooze" {
		glog.Infof("%s %d", action, alertId)
		if err := alertManager.Unsnooze(alertId, email, comment); err != nil {
			util.ReportError(w, r, err, "Failed to unsnooze alert.")
			return
		}
		return
	} else if action == "addcomment" {
		if !StringIsInteresting(comment) {
			util.ReportError(w, r, fmt.Errorf("Invalid comment text."), comment)
			return
		}
		glog.Infof("%s %d: %s", action, alertId, comment)
		if err := alertManager.AddComment(alertId, email, comment); err != nil {
			util.ReportError(w, r, err, "Failed to add comment.")
			return
		}
		return
	} else {
		util.ReportError(w, r, fmt.Errorf("Invalid action %s", action), "The requested action is invalid.")
		return
	}

}

func postAlertsJsonHandler(w http.ResponseWriter, r *http.Request) {
	// Get the alert ID.
	alertIdStr, ok := mux.Vars(r)["alertId"]
	if !ok {
		util.ReportError(w, r, fmt.Errorf("No alert ID provided."), "No alert ID provided.")
		return
	}
	alertId, err := strconv.ParseInt(alertIdStr, 10, 64)
	if err != nil {
		util.ReportError(w, r, fmt.Errorf("Invalid alert ID %s", alertIdStr), "Not found.")
		return
	}

	var req struct {
		Until   int    `json:"until"`
		Comment string `json:"comment"`
	}
	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.ReportError(w, r, err, "Failed to decode request body.")
		return
	}

	handleAlert(alertId, req.Comment, req.Until, w, r)
}

func postMultiAlertsJsonHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AlertIDs []int64 `json:"alerts"`
		Until    int     `json:"until"`
		Comment  string  `json:"comment"`
	}

	defer util.Close(r.Body)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.ReportError(w, r, err, "Failed to decode request body.")
		return
	}

	if len(req.AlertIDs) < 1 {
		util.ReportError(w, r, fmt.Errorf("No alerts specified."), "No alerts specified.")
		return
	}

	for _, a := range req.AlertIDs {
		handleAlert(a, req.Comment, req.Until, w, r)
	}
}

func handleAlerts(w http.ResponseWriter, r *http.Request, title string, categories []string, excludeCategories []string) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	categoriesJson, err := json.Marshal(categories)
	if err != nil {
		util.ReportError(w, r, fmt.Errorf("Failed to encode JSON."), "Failed to encode JSON")
	}
	excludeJson, err := json.Marshal(excludeCategories)
	if err != nil {
		util.ReportError(w, r, fmt.Errorf("Failed to encode JSON."), "Failed to encode JSON")
	}
	inp := struct {
		Categories        string
		ExcludeCategories string
		Title             string
	}{
		Categories:        string(categoriesJson),
		ExcludeCategories: string(excludeJson),
		Title:             title,
	}
	if err := alertsTemplate.Execute(w, inp); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

func alertHandler(w http.ResponseWriter, r *http.Request) {
	handleAlerts(w, r, "Skia Alerts", []string{}, []string{alerting.INFRA_ALERT})
}

func infraAlertHandler(w http.ResponseWriter, r *http.Request) {
	handleAlerts(w, r, "Skia Infra Alerts", []string{alerting.INFRA_ALERT}, []string{})
}

func rulesJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rules := struct {
		Rules []*rules.Rule `json:"rules"`
	}{
		Rules: rulesList,
	}
	if err := json.NewEncoder(w).Encode(&rules); err != nil {
		glog.Error(err)
	}
}

func rulesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}

	if err := rulesTemplate.Execute(w, struct{}{}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", alertHandler)
	r.HandleFunc("/infra", infraAlertHandler)
	r.HandleFunc("/rules", rulesHandler)
	alerts := r.PathPrefix("/json/alerts").Subrouter()
	alerts.HandleFunc("/", util.CorsHandler(alertJsonHandler))
	alerts.HandleFunc("/{alertId:[0-9]+}/{action}", postAlertsJsonHandler).Methods("POST")
	alerts.HandleFunc("/multi/{action}", postMultiAlertsJsonHandler).Methods("POST")
	r.HandleFunc("/json/rules", rulesJsonHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	defer common.LogPanic()
	alertDBConf := alerting.DBConfigFromFlags()
	buildbotDBConf := buildbot.DBConfigFromFlags()
	influxdb.SetupFlags()
	common.InitWithMetrics("alertserver", graphiteServer)
	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	Init()
	if *validateAndExit {
		if _, err := rules.MakeRules(*alertsFile, nil, time.Second, nil, *testing); err != nil {
			glog.Fatalf("Failed to set up rules: %v", err)
		}
		return
	}

	parsedPollInterval, err := time.ParseDuration(*alertPollInterval)
	if err != nil {
		glog.Fatalf("Failed to parse -alertPollInterval: %s", *alertPollInterval)
	}
	if *testing {
		*useMetadata = false
	}
	dbClient, err := influxdb.NewClientFromFlagsAndMetadata(*testing)
	if err != nil {
		glog.Fatalf("Failed to initialize InfluxDB client: %s", err)
	}
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}

	usr, err := user.Current()
	if err != nil {
		glog.Fatal(err)
	}
	tokenFile, err := filepath.Abs(usr.HomeDir + "/" + GMAIL_TOKEN_CACHE_FILE)
	if err != nil {
		glog.Fatal(err)
	}
	// By default use a set of credentials setup for localhost access.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = serverURL + "/oauth2callback/"
	var emailClientId = *emailClientIdFlag
	var emailClientSecret = *emailClientSecretFlag
	if *useMetadata {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
		emailClientId = metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_ID))
		emailClientSecret = metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_SECRET))
		cachedGMailToken := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CACHED_TOKEN))
		err = ioutil.WriteFile(tokenFile, []byte(cachedGMailToken), os.ModePerm)
		if err != nil {
			glog.Fatalf("Failed to cache token: %s", err)
		}
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST, false)

	var emailAuth *email.GMail
	if !*testing {
		if !*useMetadata && (emailClientId == "" || emailClientSecret == "") {
			glog.Fatal("If -use_metadata=false, you must provide -email_clientid and -email_clientsecret")
		}
		emailAuth, err = email.NewGMail(emailClientId, emailClientSecret, tokenFile)
		if err != nil {
			glog.Fatalf("Failed to create email auth: %v", err)
		}
	}

	// Initialize the database.
	if !*testing && *useMetadata {
		if err := alertDBConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
		if err := buildbotDBConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	if err := alertDBConf.InitDB(); err != nil {
		glog.Fatal(err)
	}
	if err := buildbotDBConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	// Create the AlertManager.
	alertManager, err = alerting.MakeAlertManager(parsedPollInterval, emailAuth)
	if err != nil {
		glog.Fatalf("Failed to create AlertManager: %v", err)
	}
	rulesList, err = rules.MakeRules(*alertsFile, dbClient, parsedPollInterval, alertManager, *testing)
	if err != nil {
		glog.Fatalf("Failed to set up rules: %v", err)
	}
	StartAlertRoutines(alertManager, 10*parsedPollInterval, dbClient)

	runServer(serverURL)
}
