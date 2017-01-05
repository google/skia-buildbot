package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

const (
	GMAIL_TOKEN_CACHE_FILE = "google_email_token.data"
	alert_email            = `Alert(s) Firing: {{range .GroupLabels}}.{{end}}

{{range .Alerts}}
  Status: {{.Status}}
  Severity: {{.Labels.severity}}
	{{.Annotations.description}}
{{end}}
`
)

// flags
var (
	emailClientIdFlag     = flag.String("email_clientid", "", "OAuth Client ID for sending email.")
	emailClientSecretFlag = flag.String("email_clientsecret", "", "OAuth Client Secret for sending email.")
	local                 = flag.Bool("local", true, "Running locally, not in prod.")
	port                  = flag.String("port", "localhost:9999", "HTTP service port (e.g., ':8001')")
	promPort              = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
)

var (
	emailAuth     *email.GMail
	emailTemplate = template.Must(template.New("alert_email").Parse(alert_email))
)

type AlertManagerRequest struct {
	Receiver string  `json:"receiver"`
	Status   string  `json:"status"`
	Alerts   []Alert `json:"alerts"`

	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`

	ExternalURL string `json:"externalURL"`
}

// Alert holds one alert for notification templates.
type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

func alertManagerHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the list of email addresses out of the url query params, i.e. ?email=foo@example.com&email=bar@example.org.
	emails := r.URL.Query()["email"]
	if len(emails) == 0 {
		httputils.ReportError(w, r, fmt.Errorf("Missing email addresses in URL: %q", r.RequestURI), "Email addresses missing.")
		return
	}

	request := AlertManagerRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httputils.ReportError(w, r, err, "Failed to decode incoming JSON")
		return
	}
	sklog.Infof("%#v", request)
	var b bytes.Buffer
	if err := emailTemplate.Execute(&b, request); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode outgoing email.")
		return
	}
	sklog.Infof("Email to send: %s", b.String())
	sklog.Infof("Email sending to: %v", emails)

	/*
		A populated requests looks like:

		main.AlertManagerRequest{
			Receiver: "general",
			Status:   "firing",
			Alerts: []main.Alert{
				main.Alert{
					Status: "firing",
					Labels: map[string]string{
						"monitor":   "codelab-monitor",
						"severity":  "warning",
						"alertname": "PerfAlert",
						"category":  "general",
						"instance":  "localhost:10110",
						"job":       "perf",
					},
					Annotations: map[string]string{
						"description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.",
						"summary":     "One or more untriaged clusters.",
					},
					StartsAt:     time.Time{sec: 63619161172, nsec: 803000000, loc: (*time.Location)(0xd269e0)},
					EndsAt:       time.Time{sec: 0, nsec: 0, loc: (*time.Location)(0xd1f240)},
					GeneratorURL: "http://prom.skia.org/graph?g0.expr=perf_clustering_untriaged+%3E+0&g0.tab=0"},
			},
			GroupLabels: map[string]string{
				"alertname": "PerfAlert",
			},
			CommonLabels:      map[string]string{"monitor": "codelab-monitor", "severity": "warning", "alertname": "PerfAlert", "category": "general", "instance": "localhost:10110", "job": "perf"},
			CommonAnnotations: map[string]string{"description": "At least one untriaged perf cluster has been found. Please visit https://perf.skia.org/t/ to triage.", "summary": "One or more untriaged clusters."},
			ExternalURL:       "https://prom.skia.org",
		}

		This should generate a single email with all the info in each of the Alerts.
	*/
}

func main() {
	defer common.LogPanic()

	common.Init()
	metrics2.InitPrometheus(*promPort)

	/*
		usr, err := user.Current()
		if err != nil {
			sklog.Fatal(err)
		}
		tokenFile, err := filepath.Abs(usr.HomeDir + "/" + GMAIL_TOKEN_CACHE_FILE)
		if err != nil {
			sklog.Fatal(err)
		}

			emailClientId := *emailClientIdFlag
			emailClientSecret := *emailClientSecretFlag
			if !*local {
				emailClientId = metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_ID))
				emailClientSecret = metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_SECRET))
				cachedGMailToken := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CACHED_TOKEN))
				err = ioutil.WriteFile(tokenFile, []byte(cachedGMailToken), os.ModePerm)
				if err != nil {
					sklog.Fatalf("Failed to cache token: %s", err)
				}
			}

			if *local && (emailClientId == "" || emailClientSecret == "") {
				sklog.Fatal("If -local, you must provide -email_clientid and -email_clientsecret")
			}
			emailAuth, err = email.NewGMail(emailClientId, emailClientSecret, tokenFile)
			if err != nil {
				sklog.Fatalf("Failed to create email auth: %v", err)
			}
	*/

	// Resources are served directly.
	router := mux.NewRouter()

	router.HandleFunc("/alertmanager", alertManagerHandler).Methods("POST")

	http.Handle("/", httputils.LoggingGzipRequestResponse(router))

	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
