package main

/*
	This is a server which collects and serves information about Task Drivers.
*/

import (
	"context"
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/db/memory"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	SUBSCRIPTION_NAME = "td_server_collector"
)

var (
	// Flags.
	host         = flag.String("host", "localhost", "HTTP service host")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	project      = flag.String("project_id", "", "GCE Project ID")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "./dist", "The directory to find templates, JS, and CSS files. If blank the \"dist\" subdirectory of the current directory will be used.")

	// Database used for storing and retrieving Task Drivers.
	d db.DB

	// HTML templates.
	tdTemplate *template.Template = nil
)

// getTaskDriver returns a display.TaskDriverRunDisplay instance for the given
// request. If anything went wrong, returns nil and writes an error to the
// ResponseWriter.
func getTaskDriver(w http.ResponseWriter, r *http.Request) *display.TaskDriverRunDisplay {
	id, ok := mux.Vars(r)["id"]
	if !ok {
		http.Error(w, "No task driver ID in request path.", http.StatusBadRequest)
		return nil
	}
	td, err := d.GetTaskDriver(id)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve task driver.")
		return nil
	}
	if td == nil {
		http.Error(w, "No task driver exists with the given ID.", http.StatusNotFound)
		return nil
	}
	disp, err := display.TaskDriverForDisplay(td)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to format task driver for response.")
		return nil
	}
	return disp
}

// taskDriverHandler handles requests for an individual Task Driver.
func taskDriverHandler(w http.ResponseWriter, r *http.Request) {
	disp := getTaskDriver(w, r)
	if disp == nil {
		// Any error was handled by getTaskDriver.
		return
	}

	if *local {
		// reload during local development
		loadTemplates()
	}
	b, err := json.Marshal(disp)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
		return
	}
	page := struct {
		TaskName string
		TaskJson string
	}{
		TaskName: disp.Name,
		TaskJson: string(b),
	}
	w.Header().Set("Content-Type", "text/html")
	if err := tdTemplate.Execute(w, page); err != nil {
		httputils.ReportError(w, r, err, "Server could not load page")
		return
	}
}

// jsonTaskDriverHandler returns the JSON representation of the requested Task Driver.
func jsonTaskDriverHandler(w http.ResponseWriter, r *http.Request) {
	disp := getTaskDriver(w, r)
	if disp == nil {
		// Any error was handled by getTaskDriver.
		return
	}

	if err := json.NewEncoder(w).Encode(disp); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode response.")
		return
	}
}

// Load the HTML pages.
func loadTemplates() {
	tdTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "task-driver-index.html"),
	))
}

// Run the web server.
func runServer(ctx context.Context, serverURL string) {
	loadTemplates()
	r := mux.NewRouter()
	r.HandleFunc("/td/{id}", taskDriverHandler)
	r.HandleFunc("/json/td/{id}", jsonTaskDriverHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))))
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	sklog.AddLogsRedirect(r)
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

// Entry mimics logging.Entry, which for some reason does not include the
// jsonPayload field, and is not parsable via json.Unmarshal due to the Severity
// type.
type Entry struct {
	Labels      map[string]string `json:"labels"`
	JsonPayload td.Message        `json:"jsonPayload"`
}

// handleMessage decodes and inserts an update
func handleMessage(msg *pubsub.Message) error {
	var e Entry
	if err := json.Unmarshal(msg.Data, &e); err != nil {
		// If the message has badly-formatted data,
		// we'll never be able to parse it, so go ahead
		// and ack it to get it out of the queue.
		msg.Ack()
		return err
	}
	if err := db.UpdateFromMessage(d, &e.JsonPayload); err != nil {
		// This may be a transient error, so nack the message and hope
		// that we'll be able to handle it on redelivery.
		msg.Nack()
		return err
	}
	msg.Ack()
	return nil
}

func main() {
	common.InitWithMust(
		"task-driver-server",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()
	if *project == "" {
		sklog.Fatal("--project_id is required.")
	}
	skiaversion.MustLogVersion()

	// Setup pubsub.
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, *project)
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic(td.PUBSUB_TOPIC)
	if exists, err := topic.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if !exists {
		topic, err = client.CreateTopic(ctx, td.PUBSUB_TOPIC)
		if err != nil {
			sklog.Fatal(err)
		}
	}
	sub := client.Subscription(SUBSCRIPTION_NAME)
	if exists, err := sub.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if !exists {
		sub, err = client.CreateSubscription(ctx, SUBSCRIPTION_NAME, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 10 * time.Second,
		})
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Create the TaskDriver DB.
	d = memory.NewInMemoryDB()

	// Launch a goroutine to listen for pubsub messages.
	go func() {
		sklog.Infof("Waiting for messages.")
		for {
			if err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				if err := handleMessage(msg); err != nil {
					sklog.Errorf("Failed to handle pubsub message: %s", err)
				}
			}); err != nil {
				sklog.Fatal(err)
			}
		}
	}()

	// Run the web server.
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	runServer(ctx, serverURL)
}
