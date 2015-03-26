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
	"github.com/skia-dev/influxdb/client"
)

import (
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/alertserver/go/rules"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
)

const (
	GMAIL_TOKEN_CACHE_FILE = "google_email_token.data"
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
	influxDbHost          = flag.String("influxdb_host", "localhost:8086", "The InfluxDB hostname.")
	influxDbName          = flag.String("influxdb_name", "root", "The InfluxDB username.")
	influxDbPassword      = flag.String("influxdb_password", "root", "The InfluxDB password.")
	influxDbDatabase      = flag.String("influxdb_database", "", "The InfluxDB database.")
	emailClientIdFlag     = flag.String("email_clientid", "", "OAuth Client ID for sending email.")
	emailClientSecretFlag = flag.String("email_clientsecret", "", "OAuth Client Secret for sending email.")
	alertPollInterval     = flag.String("alert_poll_interval", "1s", "How often to check for new alerts.")
	alertsFile            = flag.String("alerts_file", "alerts.cfg", "Config file containing alert rules.")
	testing               = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
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

func alertJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := alertManager.WriteActiveAlertsJson(w); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Unable to write JSON: %v", err))
	}
}

func postAlertsJsonHandler(w http.ResponseWriter, r *http.Request) {
	email := login.LoggedInAs(r)
	if !userHasEditRights(email) {
		util.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "You must be logged in to an account with edit rights to do that.")
		return
	}

	// Get the alert ID.
	alertIdStr, ok := mux.Vars(r)["alertId"]
	if !ok {
		util.ReportError(w, r, fmt.Errorf("No alert ID provided."), "No alert ID provided.")
	}
	alertId, err := strconv.ParseInt(alertIdStr, 10, 64)
	if err != nil {
		util.ReportError(w, r, fmt.Errorf("Invalid alert ID %s", alertIdStr), "Not found.")
	}

	action, ok := mux.Vars(r)["action"]
	if !ok {
		util.ReportError(w, r, fmt.Errorf("No action provided."), "No action provided.")
	}

	if action == "dismiss" {
		glog.Infof("%s %d", action, alertId)
		if err := alertManager.Dismiss(alertId, email, ""); err != nil {
			util.ReportError(w, r, err, "Failed to dismiss alert.")
			return
		}
		return
	} else if action == "snooze" {
		d := json.NewDecoder(r.Body)
		body := struct {
			Until int
		}{}
		err := d.Decode(&body)
		if err != nil || body.Until == 0 {
			util.ReportError(w, r, err, fmt.Sprintf("Unable to decode request body: %s", r.Body))
			return
		}
		defer util.Close(r.Body)
		until := time.Unix(int64(body.Until), 0)
		glog.Infof("%s %d until %v", action, alertId, until.String())
		if err := alertManager.Snooze(alertId, until, email); err != nil {
			util.ReportError(w, r, err, "Failed to snooze alert.")
			return
		}
		return
	} else if action == "unsnooze" {
		glog.Infof("%s %d", action, alertId)
		if err := alertManager.Unsnooze(alertId, email); err != nil {
			util.ReportError(w, r, err, "Failed to unsnooze alert.")
			return
		}
		return
	} else if action == "addcomment" {
		c := struct {
			Comment string `json:"comment"`
		}{}
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			util.ReportError(w, r, err, fmt.Sprintf("Unable to read request body: %s", r.Body))
			return
		}
		defer util.Close(r.Body)
		if !StringIsInteresting(c.Comment) {
			util.ReportError(w, r, fmt.Errorf("Invalid comment text."), c.Comment)
			return
		}
		glog.Infof("%s %d: %s", action, alertId, c.Comment)
		if err := alertManager.AddComment(alertId, email, c.Comment); err != nil {
			util.ReportError(w, r, err, "Failed to add comment.")
			return
		}
		return
	} else {
		util.ReportError(w, r, fmt.Errorf("Invalid action %s", action), "The requested action is invalid.")
		return
	}

}

func alertHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *testing {
		reloadTemplates()
	}
	if err := alertsTemplate.Execute(w, struct{}{}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
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
	r.HandleFunc("/rules", rulesHandler)
	alerts := r.PathPrefix("/json/alerts").Subrouter()
	alerts.HandleFunc("/", alertJsonHandler)
	alerts.HandleFunc("/{alertId:[0-9]+}/{action}", postAlertsJsonHandler).Methods("POST")
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
	database.SetupFlags(alerting.PROD_DB_HOST, alerting.PROD_DB_PORT, database.USER_RW, alerting.PROD_DB_NAME)
	common.InitWithMetrics("alertserver", graphiteServer)
	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	Init()
	parsedPollInterval, err := time.ParseDuration(*alertPollInterval)
	if err != nil {
		glog.Fatalf("Failed to parse -alertPollInterval: %s", *alertPollInterval)
	}
	if *testing {
		*useMetadata = false
	}
	if *useMetadata {
		*influxDbName = metadata.Must(metadata.ProjectGet(metadata.INFLUXDB_NAME))
		*influxDbPassword = metadata.Must(metadata.ProjectGet(metadata.INFLUXDB_PASSWORD))
	}
	dbClient, err := client.New(&client.ClientConfig{
		Host:       *influxDbHost,
		Username:   *influxDbName,
		Password:   *influxDbPassword,
		Database:   *influxDbDatabase,
		HttpClient: nil,
		IsSecure:   false,
		IsUDP:      false,
	})
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
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, login.DEFAULT_SCOPE)

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
	conf, err := database.ConfigFromFlagsAndMetadata(*testing, alerting.MigrationSteps())
	if err != nil {
		glog.Fatal(err)
	}
	if err := alerting.InitDB(conf); err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Database config: %s", conf.MySQLString)

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
