package emailservice

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	serverReadTimeout  = 5 * time.Minute
	serverWriteTimeout = 5 * time.Minute
)

type getClientFromKeyType func(ctx context.Context, email, key string) (*email.GMail, error)

// App is the main email service application.
type App struct {
	// flags
	local      bool
	port       string
	project    string
	promPort   string
	secretName string

	sendgridClient *sendgrid.Client
	sendSucces     metrics2.Counter
	sendFailure    metrics2.Counter
}

// Flagset constructs a flag.FlagSet for the App.
func (a *App) Flagset() *flag.FlagSet {
	fs := flag.NewFlagSet("emailservice", flag.ExitOnError)
	fs.BoolVar(&a.local, "local", false, "Running locally if true. As opposed to in production.")
	fs.StringVar(&a.port, "port", ":8000", "HTTP service address (e.g., ':8000')")
	fs.StringVar(&a.project, "project", "skia-public", "The GCP Project that holds the secret.")
	fs.StringVar(&a.secretName, "secret-name", "sendgrid-api-key", "The name of the GCP secret that contains the SendGrid API key..")
	fs.StringVar(&a.promPort, "prom-port", ":20000", "Metrics service address (e.g., ':10110')")

	return fs
}

// New returns a new instance of App.
func New(ctx context.Context) (*App, error) {
	var ret App

	err := common.InitWith(
		"email-service",
		common.CloudLogging(&ret.local, "skia-public"),
		common.MetricsLoggingOpt(),
		common.PrometheusOpt(&ret.promPort),
		common.FlagSetOpt(ret.Flagset()),
	)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed common.InitWith().")
	}

	secretClient, err := secret.NewClient(context.Background())
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed creating secret client")
	}
	sendGridAPIKey, err := secretClient.Get(ctx, ret.project, ret.secretName, secret.VersionLatest)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed retrieving secret: %q from project: %q", ret.secretName, ret.project)
	}
	sklog.Infof("API Key retrieved.")

	ret.sendSucces = metrics2.GetCounter("emailservice_send_success")
	ret.sendFailure = metrics2.GetCounter("emailservice_send_failure")
	ret.sendgridClient = sendgrid.NewSendClient(sendGridAPIKey)
	return &ret, nil
}

func (a *App) reportSendError(w http.ResponseWriter, err error, msg string) {
	httputils.ReportError(w, err, msg, http.StatusBadRequest)
	a.sendFailure.Inc(1)
}

// Handle incoming POST's of RFC2822 formatted emails, which are then parsed
// and sent.
func (a *App) incomingEmaiHandler(w http.ResponseWriter, r *http.Request) {
	// Read the entire RFC2822 body.
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		a.reportSendError(w, err, "Failed to read body.")
		return
	}
	bodyAsString := string(body)

	sklog.Infof("Received: %q", bodyAsString)

	from, to, subject, htmlBody, err := email.ParseRFC2822Message(body)
	if err != nil {
		a.reportSendError(w, err, "Failed to parse RFC 2822 body.")
		return
	}

	parsedFrom, err := mail.ParseEmail(from)
	if err != nil {
		a.reportSendError(w, err, "Failed to parse From: address.")
		return
	}
	parsedTo, err := mail.ParseEmail(to)
	if err != nil {
		a.reportSendError(w, err, "Failed to parse To: address.")
		return
	}

	resp, err := a.sendgridClient.Send(mail.NewSingleEmail(parsedFrom, subject, parsedTo, "", htmlBody))
	if err != nil {
		a.reportSendError(w, err, fmt.Sprintf("Failed to send via API: %q", resp.Body))
		return
	}
	sklog.Infof("Successfully sent from: %q to: %q", from, to)
}

// Run the email service. This function will only return on failure.
func (a *App) Run() error {

	// Add all routing.
	r := mux.NewRouter()
	r.HandleFunc("/send", a.incomingEmaiHandler).Methods("POST")

	// We must specify that we handle /healthz or it will never flow through to our middleware.
	// Even though this handler is never actually called (due to the early termination in
	// httputils.HealthzAndHTTPS), we need to have it added to the routes we handle.
	r.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	sklog.Infof("Ready to serve on port %s", a.port)
	server := &http.Server{
		Addr:           a.port,
		Handler:        r,
		ReadTimeout:    serverReadTimeout,
		WriteTimeout:   serverWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}

// Main runs the application. This function will only return on failure.
func Main() error {
	app, err := New(context.Background())
	if err != nil {
		return skerr.Wrap(err)
	}

	return app.Run()
}
