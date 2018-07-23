// webhook_proxy takes POST'd JSON requests from various sources, such as Prometheus
// AlertManager and turns them into outgoing emails.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/gorilla/mux"

	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/prometheus/go/alertmanager"
)

const (
	GMAIL_TOKEN_CACHE_FILE = "google_email_token.data"
	FROM_ADDRESS           = "alertserver@skia.org"
)

// flags
var (
	emailClientIdFlag     = flag.String("email_clientid", "", "OAuth Client ID for sending email.")
	emailClientSecretFlag = flag.String("email_clientsecret", "", "OAuth Client Secret for sending email.")
	local                 = flag.Bool("local", false, "Running locally, not in prod.")
	port                  = flag.String("port", "localhost:8004", "HTTP service port (e.g., ':8001')")
	publicPort            = flag.String("public_port", ":8005", "HTTP service port (e.g., ':8001')")
	promPort              = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
)

var (
	emailAuth *email.GMail
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
		httputils.ReportError(w, r, err, "Failed to encode outgoing email.")
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
		httputils.ReportError(w, r, err, "Failed to encode outgoing chat.")
		return
	}

	// Send the message to the chat room.
	if err := chatbot.Send(body, to, ""); err != nil {
		httputils.ReportError(w, r, err, "Failed to send outgoing chat.")
		return
	}
}

// publicWebhookHandler accepts incoming webhook requests on a
// publicly exposed endpoint.
//
// At this point we just log the contents. More functionality TBD.
func publicWebhookHandler(w http.ResponseWriter, r *http.Request) {
	botName := mux.Vars(r)["bot"]
	sklog.Infof("Webhook: Bot: %q URL %#v Host: %q  URI: %q", botName, *(r.URL), r.Host, r.RequestURI)
	b, err := ioutil.ReadAll(r.Body)
	if err == nil {
		sklog.Infof("Body: %q", string(b))
	} else {
		sklog.Errorf("Error reading webhook body: %s", err)
	}
}

func main() {

	common.InitWithMust(
		"webhook_proxy",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	chatbot.Init("AlertManager")

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

	// Resources are served directly.
	router := mux.NewRouter()

	router.HandleFunc("/email", emailHandler).Methods("POST")
	router.HandleFunc("/chat", chatHandler).Methods("POST")

	http.Handle("/", httputils.LoggingGzipRequestResponse(router))

	sklog.Infoln("Ready to serve.")
	go func() {
		sklog.Fatal(http.ListenAndServe(*port, nil))
	}()

	r := mux.NewRouter()
	r.HandleFunc("/h/{bot}", publicWebhookHandler).Methods("POST")
	glog.Fatal(http.ListenAndServe(*publicPort, httputils.LoggingGzipRequestResponse(r)))
}
