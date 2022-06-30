package emailservice

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

// App is the main email service application.
type App struct {
	// flags
	port       string
	project    string
	promPort   string
	secretName string

	sendgridClient *sendgrid.Client
	sendSuccess    metrics2.Counter
	sendFailure    metrics2.Counter
}

// Flagset constructs a flag.FlagSet for the App.
func (a *App) Flagset() *flag.FlagSet {
	fs := flag.NewFlagSet("emailservice", flag.ExitOnError)
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

	ret.sendSuccess = metrics2.GetCounter("emailservice_send_success")
	ret.sendFailure = metrics2.GetCounter("emailservice_send_failure")
	ret.sendgridClient = sendgrid.NewSendClient(sendGridAPIKey)
	return &ret, nil
}

func (a *App) reportSendError(w http.ResponseWriter, err error, msg string) {
	httputils.ReportError(w, err, msg, http.StatusBadRequest)
	a.sendFailure.Inc(1)
}

func convertRFC2822ToSendGrid(r io.Reader) (*mail.SGMailV3, error) {
	// Parse the entire incoming RFC2822 body.
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read body.")
	}
	bodyAsString := string(body)

	sklog.Infof("Received: %q", bodyAsString)

	from, to, subject, htmlBody, err := email.ParseRFC2822Message(body)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse RFC 2822 body.")
	}

	// Parse the From: line.
	parsedFrom, err := mail.ParseEmail(from)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse From: address.")
	}

	m := mail.NewV3Mail()
	m.SetFrom(parsedFrom)
	m.Subject = subject

	// Parse the To: line.
	p := mail.NewPersonalization()
	tos := []*mail.Email{}
	for _, addr := range to {
		parsedTo, err := mail.ParseEmail(addr)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to parse To: address.")
		}
		tos = append(tos, parsedTo)
	}
	p.AddTos(tos...)
	m.AddPersonalizations(p)

	c := mail.NewContent("text/html", htmlBody)
	m.AddContent(c)
	return m, nil
}

// Error is a single error returned in a Response.
type Error struct {
	Message string `json:"message"`
	Field   string `json:"field"`
	Help    string `json:"help"`
}

// Response is the JSON format of the body the SendGrid API returns.
type Response struct {
	Errors []Error `json:"errors,omitempty"`
}

// Handle incoming POST's of RFC2822 formatted emails, which are then parsed and
// sent.
func (a *App) incomingEmaiHandler(w http.ResponseWriter, r *http.Request) {
	m, err := convertRFC2822ToSendGrid(r.Body)
	if err != nil {
		a.reportSendError(w, err, "Failed to convert RFC2822 body to SendGrid API format")
		return
	}

	resp, err := a.sendgridClient.Send(m)
	if err != nil {
		a.reportSendError(w, err, "Failed to send via API")
		return
	}

	sklog.Infof("Response Body: %q", resp.Body)
	sklog.Infof("Response Headers: %s", resp.Headers)

	if h, ok := resp.Headers["X-Message-Id"]; ok && len(h) > 0 {
		w.Header().Set("X-Message-Id", h[0])
	}
	var decodedResponse Response
	if err := json.Unmarshal([]byte(resp.Body), &decodedResponse); err != nil {
		sklog.Warningf("Failed to decode JSON: %s", err)
	}
	if len(decodedResponse.Errors) > 0 {
		a.reportSendError(w, err, fmt.Sprintf("Failed to send via API: %q", resp.Body))
		return
	}

	sklog.Infof("Successfully sent from: %q", m.From.Address)
	a.sendSuccess.Inc(1)
}

// Run the email service. This function will only return on failure.
func (a *App) Run() error {

	// Add all routing.
	r := mux.NewRouter()
	r.HandleFunc("/send", a.incomingEmaiHandler).Methods("POST")

	// We must specify that we handle /healthz or it will never flow through to
	// our middleware. Even though this handler is never actually called (due to
	// the early termination in httputils.HealthzAndHTTPS), we need to have it
	// added to the routes we handle.
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
