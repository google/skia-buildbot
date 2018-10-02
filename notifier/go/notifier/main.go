// notifier takes POST'd JSON requests from various sources, such as Prometheus
// AlertManager and turns them into outgoing emails.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/prometheus/go/alertmanager"
)

const (
	FROM_ADDRESS = "alertserver@skia.org"
)

// flags
var (
	emailClientSecretFile = flag.String("email_client_secret_file", "/etc/alertmanger_webhooks/email/client_secret.json", "OAuth client secret JSON file for sending email.")
	emailTokenCacheFile   = flag.String("email_token_cache_file", "/var/alertmanger_webhooks/client_token.json", "OAuth token cache file for sending email.")
	chatWebHooksFile      = flag.String("chat_webhooks_file", "/etc/alertmanager_webhooks/chat/chat_config.txt", "Chat webhook config.")
	local                 = flag.Bool("local", false, "Running locally, not in prod.")
	port                  = flag.String("port", ":8000", "HTTP service port (e.g., ':8001')")
	promPort              = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

var (
	emailAuth *email.GMail

	chatBotConfigReader chatbot.ConfigReader
)

// emailHandler accepts incoming JSON encoded alertmanager.AlertManagerRequest's and sends
// emails based off that content.
//
// Email addresses are supplied as query parameters, and there can be more than
// one, i.e. ?email=foo@example.com&email=bar@example.org.
func emailHandler(w http.ResponseWriter, r *http.Request) {
	to := r.URL.Query()["email"]
	sklog.Infof("Sending to: %q", to)
	if len(to) == 0 {
		httputils.ReportError(w, r, fmt.Errorf("Missing email addresses in URL: %q", r.RequestURI), "Email addresses missing.")
		return
	}
	body, subject, err := alertmanager.Email(r.Body)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to generate an outgoing email.")
		return
	}

	if err := emailAuth.Send(FROM_ADDRESS, to, subject, body); err != nil {
		httputils.ReportError(w, r, err, "Failed to send outgoing email.")
		return
	}
}

// chatHandler accepts incoming JSON encoded alertmanager.AlertManagerRequest's and sends
// messages based off that content to the chat webhook proxy.
//
// The query parameter 'room' should indicate the room to send the message to.
func chatHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the chat room name out of the url query params, i.e. ?room=skiabot_alerts.
	to := r.URL.Query().Get("room")
	if to == "" {
		httputils.ReportError(w, r, fmt.Errorf("Missing room in URL: %q", r.RequestURI), "Chat room name missing.")
		return
	}

	// Compose the message.
	body, err := alertmanager.Chat(r.Body)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to generate an outgoing chat.")
		return
	}

	year, week := time.Now().ISOWeek()
	threadId := fmt.Sprintf("%d-%d", week, year)

	// Send the message to the chat room.
	if err := chatbot.SendUsingConfig(body, to, threadId, chatBotConfigReader); err != nil {
		httputils.ReportError(w, r, err, "Failed to send outgoing chat.")
		return
	}
}

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Installed struct {
	Installed ClientConfig `json:"installed"`
}

func main() {
	common.InitWithMust(
		"notifier",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	// Init chatbot.
	chatbot.Init("AlertManager")
	chatBotConfigReader = func() string {
		if b, err := ioutil.ReadFile(*chatWebHooksFile); err != nil {
			sklog.Errorf("Failed to read chat config %q: %s", *chatWebHooksFile, err)
			return ""
		} else {
			return string(b)
		}
	}
	var err error
	emailAuth, err = email.NewFromFiles(*emailTokenCacheFile, *emailClientSecretFile)
	if err != nil {
		sklog.Fatalf("Failed to create email auth: %v", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/email", emailHandler).Methods("POST")
	router.HandleFunc("/chat", chatHandler).Methods("POST")
	router.HandleFunc("/healthz", httputils.HealthCheckHandler).Methods("GET")
	http.Handle("/", httputils.LoggingGzipRequestResponse(router))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
