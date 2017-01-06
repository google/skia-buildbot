// webhook_email_proxy takes POST'd JSON requests from the Prometheus
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

	"github.com/gorilla/mux"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/metrics2"
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
	port                  = flag.String("port", "localhost:9999", "HTTP service port (e.g., ':8001')")
	promPort              = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
)

var (
	emailAuth *email.GMail
)

func alertManagerHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the list of email addresses out of the url query params, i.e. ?email=foo@example.com&email=bar@example.org.
	to := r.URL.Query()["email"]
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

func main() {
	defer common.LogPanic()

	common.Init()
	metrics2.InitPrometheus(*promPort)

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

	router.HandleFunc("/alertmanager", alertManagerHandler).Methods("POST")

	http.Handle("/", httputils.LoggingGzipRequestResponse(router))

	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
