/*
	Provides roll-up statuses and alerting for Skia build/test/perf.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

import (
	"github.com/fiorix/go-web/autogzip"
	"github.com/golang/glog"
	"github.com/influxdb/influxdb/client"
)

import (
	"skia.googlesource.com/buildbot.git/go/email"
	"skia.googlesource.com/buildbot.git/go/login"
	"skia.googlesource.com/buildbot.git/go/metadata"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/monitoring/go/alerting"
)

const (
	COOKIESALT_METADATA_KEY          = "cookiesalt"
	CLIENT_ID_METADATA_KEY           = "clientid"
	CLIENT_SECRET_METADATA_KEY       = "clientsecret"
	INFLUXDB_NAME_METADATA_KEY       = "influxdb_name"
	INFLUXDB_PASSWORD_METADATA_KEY   = "influxdb_password"
	GMAIL_CLIENT_ID_METADATA_KEY     = "gmail_clientid"
	GMAIL_CLIENT_SECRET_METADATA_KEY = "gmail_clientsecret"
	GMAIL_CACHED_TOKEN_METADATA_KEY  = "gmail_cached_token"
	GMAIL_TOKEN_CACHE_FILE           = "google_email_token.data"
)

var (
	alertManager *alerting.AlertManager = nil
)

// flags
var (
	host                  = flag.String("host", "localhost", "HTTP service host")
	port                  = flag.String("port", "8001", "HTTP service port (e.g., '8001')")
	useMetadata           = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	influxDbHost          = flag.String("influxdb_host", "localhost:8086", "The InfluxDB hostname.")
	influxDbName          = flag.String("influxdb_name", "root", "The InfluxDB username.")
	influxDbPassword      = flag.String("influxdb_password", "root", "The InfluxDB password.")
	influxDbDatabase      = flag.String("influxdb_database", "", "The InfluxDB database.")
	emailClientIdFlag     = flag.String("email_clientid", "", "OAuth Client ID for sending email.")
	emailClientSecretFlag = flag.String("email_clientsecret", "", "OAuth Client Secret for sending email.")
	alertPollInterval     = flag.String("alert_poll_interval", "1s", "How often to check for new alerts.")
	alertsFile            = flag.String("alerts_file", "alerts.cfg", "Config file containing alert rules.")
)

func userHasEditRights(r *http.Request) bool {
	email := login.LoggedInAs(r)
	if strings.HasSuffix(email, "@google.com") {
		return true
	}
	return false
}

func alertJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	type displayAlert struct {
		Id           string `json:"id"`
		Name         string `json:"name"`
		Query        string `json:"query"`
		Condition    string `json:"condition"`
		Active       bool   `json:"active"`
		Snoozed      bool   `json:"snoozed"`
		Triggered    int32  `json:"triggered"`
		SnoozedUntil int32  `json:"snoozedUntil"`
	}
	alerts := struct {
		Alerts []displayAlert `json:"alerts"`
	}{
		Alerts: []displayAlert{},
	}
	for _, a := range alertManager.Alerts() {
		alerts.Alerts = append(alerts.Alerts, displayAlert{
			Id:           a.Id,
			Name:         a.Name,
			Query:        a.Query,
			Condition:    a.Condition,
			Active:       a.Active(),
			Snoozed:      a.Snoozed(),
			Triggered:    int32(a.Triggered().Unix()),
			SnoozedUntil: int32(a.SnoozedUntil().Unix()),
		})
	}
	bytes, err := json.Marshal(&alerts)
	if err != nil {
		glog.Error(err)
	}
	w.Write(bytes)
}

func alertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if !userHasEditRights(r) {
			util.ReportError(w, r, fmt.Errorf("User does not have edit rights."), "You must be logged in to an account with edit rights to do that.")
			return
		}
		// URLs take the form /alerts/<alertId>/<action>
		// TODO(borenet): Ensure user is logged-in and authorized to do this!
		split := strings.Split(r.URL.String(), "/")
		if len(split) != 4 {
			util.ReportError(w, r, fmt.Errorf("Invalid URL %s", r.URL), "Requested URL is not valid.")
			return
		}
		alertId := split[2]
		if !alertManager.Contains(alertId) {
			util.ReportError(w, r, fmt.Errorf("Invalid Alert ID %d", alertId), "The requested resource does not exist.")
			return
		}
		action := split[3]
		if action == "dismiss" {
			glog.Infof("%s %s", action, alertId)
			alertManager.Dismiss(alertId)
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
			until := time.Unix(int64(body.Until), 0)
			glog.Infof("%s %s until %v", action, alertId, until.String())
			alertManager.Snooze(alertId, until)
			return
		} else if action == "unsnooze" {
			glog.Infof("%s %s", action, alertId)
			alertManager.Unsnooze(alertId)
			return
		} else {
			util.ReportError(w, r, fmt.Errorf("Invalid action %s", action), "The requested action is invalid.")
			return
		}
	}
	http.ServeFile(w, r, "res/html/alerts.html")
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir("./"))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

func runServer(serverURL string) {
	_, filename, _, _ := runtime.Caller(0)
	cwd := filepath.Join(filepath.Dir(filename), "../..")
	if err := os.Chdir(cwd); err != nil {
		glog.Fatal(err)
	}

	http.HandleFunc("/res/", autogzip.HandleFunc(makeResourceHandler()))
	http.HandleFunc("/", alertHandler)
	http.HandleFunc("/json/alerts", alertJsonHandler)
	http.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	http.HandleFunc("/logout/", login.LogoutHandler)
	http.HandleFunc("/loginstatus/", login.StatusHandler)
	glog.Infof("Ready to serve on http://%s", serverURL)
	glog.Fatal(http.ListenAndServe(serverURL, nil))
}

func main() {
	flag.Parse()
	defer glog.Flush()
	parsedPollInterval, err := time.ParseDuration(*alertPollInterval)
	if err != nil {
		glog.Fatal(fmt.Sprintf("Failed to parse -alertPollInterval: %s", *alertPollInterval))
	}
	if *useMetadata {
		*influxDbName = metadata.MustGet(INFLUXDB_NAME_METADATA_KEY)
		*influxDbPassword = metadata.MustGet(INFLUXDB_PASSWORD_METADATA_KEY)
	}
	dbClient, err := client.New(&client.ClientConfig{*influxDbHost, *influxDbName, *influxDbPassword, *influxDbDatabase, nil, false, false})
	if err != nil {
		glog.Fatal(fmt.Sprintf("Failed to initialize InfluxDB client: %s", err))
	}
	serverURL := *host + ":" + *port

	tokenFile, err := filepath.Abs(GMAIL_TOKEN_CACHE_FILE)
	if err != nil {
		glog.Fatal(err)
	}
	// By default use a set of credentials setup for localhost access.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = "http://" + serverURL + "/oauth2callback/"
	var emailClientId = *emailClientIdFlag
	var emailClientSecret = *emailClientSecretFlag
	if *useMetadata {
		cookieSalt = metadata.MustGet(COOKIESALT_METADATA_KEY)
		clientID = metadata.MustGet(CLIENT_ID_METADATA_KEY)
		clientSecret = metadata.MustGet(CLIENT_SECRET_METADATA_KEY)
		emailClientId = metadata.MustGet(GMAIL_CLIENT_ID_METADATA_KEY)
		emailClientSecret = metadata.MustGet(GMAIL_CLIENT_SECRET_METADATA_KEY)
		cachedGMailToken := metadata.MustGet(GMAIL_CACHED_TOKEN_METADATA_KEY)
		err = ioutil.WriteFile(tokenFile, []byte(cachedGMailToken), os.ModePerm)
		if err != nil {
			glog.Fatalf("Failed to cache token: %s", err)
		}
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt)

	var emailAuth *email.GMail
	if !*useMetadata && (emailClientId == "" || emailClientSecret == "") {
		glog.Fatal("If -use_metadata=false, you must provide -email_clientid and -email_clientsecret")
	}
	emailAuth, err = email.NewGMail(emailClientId, emailClientSecret, tokenFile)
	if err != nil {
		glog.Fatal(fmt.Sprintf("Failed to create email auth: %v", err))
	}
	alertManager, err = alerting.NewAlertManager(dbClient, *alertsFile, parsedPollInterval, emailAuth)
	if err != nil {
		glog.Fatal(fmt.Sprintf("Failed to create AlertManager: %v", err))
	}

	runServer(serverURL)
}
