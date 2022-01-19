package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"text/template"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"google.golang.org/api/option"

	"go.skia.org/infra/codesize/go/bloaty"
	"go.skia.org/infra/codesize/go/codesizeserver/rpc"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	gcpProjectName = "skia-public"
	gcsBucket      = "skia-codesize"
	pubSubTopic    = "skia-codesize-files"
)

// gcsPubSubEvent is the payload of the PubSub events sent by GCS on file uploads. See
// https://cloud.google.com/storage/docs/json_api/v1/objects#resource-representations.
type gcsPubSubEvent struct {
	// Bucket name, e.g. "skia-codesize".
	Bucket string `json:"bucket"`
	// Name of the affected file (relative to the bucket), e.g. "foo/bar/baz.txt".
	Name string `json:"name"`
}

type server struct {
	templates *template.Template
	gcsClient *gcsclient.StorageClient

	// bloatyFile holds the contents of a single Bloaty file loaded from GCS, and will soon be
	// replaced with a more appropriate data structure to support multiple artifacts and metadata
	// (from JSON files with build parameters, Bloaty command-line arguments, etc.).
	//
	// TODO(lovisolo): Replace with a more definitive in-memory cache with the above information.
	bloatyFile string
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	ctx := context.Background()
	srv := &server{}

	// Set up GCS client.
	ts, err := auth.NewDefaultTokenSource(*baseapp.Local, storage.ScopeReadWrite)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get token source")
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create storage client")
	}
	srv.gcsClient = gcsclient.New(storageClient, gcsBucket)

	// Preload the latest Bloaty outputs.
	if err := srv.preloadBloatyFiles(); err != nil {
		return nil, skerr.Wrapf(err, "failed to preload Bloaty outputs from GCS")
	}

	// Subscribe to the PubSub topic via which GCS will notify us of file uploads. We use a broadcast
	// name provider to ensure that all replicas are notified of each file upload. We specify an
	// expiration policy to shorten the time until unused subscriptions are garbage-collected after
	// redeploying the service.
	expirationPolicy := time.Hour * 24 * 7
	subscription, err := sub.NewWithSubNameProviderAndExpirationPolicy(ctx, *baseapp.Local, gcpProjectName, pubSubTopic, sub.NewBroadcastNameProvider(*baseapp.Local, pubSubTopic), &expirationPolicy, 1)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to subscribe to PubSub topic")
	}

	// Launch a goroutine to listen for PubSub messages.
	go func() {
		sklog.Infof("Listening for PubSub messages.")
		for {
			if err := subscription.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				evt := gcsPubSubEvent{}
				if err := json.Unmarshal(msg.Data, &evt); err != nil {
					// This should never happen. But if it does, then GCS is sending spurious messages, or
					// something else is publishing on the topic.
					msg.Ack() // No point in retrying a spurious message.
					// TODO(lovisolo): Add a metrics counter.
					sklog.Errorf("Received malformed PubSub message. JSON Unmarshal error: %s\n", err)
					return
				}

				sklog.Infof("Received PubSub message: [bucket: %s, name: %s].\n", evt.Bucket, evt.Name)

				// We should only receive events for files in our GCS bucket.
				if evt.Bucket != gcsBucket {
					msg.Ack() // No point in retrying a spurious message.
					// TODO(lovisolo): Add a metrics counter.
					sklog.Errorf("Received a PubSub message from an unknown GCS bucket %q, but %q was expected. Potential configuration error.", evt.Bucket, gcsBucket)
					return
				}

				// Process message.
				if err := srv.handleFileUploadNotification(ctx, evt.Name); err != nil {
					// TODO(lovisolo): Add a metrics counter.
					sklog.Warningf("Failed to process PubSub message: [bucket: %s, name: %s]. Error: %s.\n", evt.Bucket, evt.Name, err)

					// We Ack messages we failed to process to prevent them from being continuously retried.
					// Some of these errors might be retriable, such as transiet network errors while
					// downloading from GCS, but I don't anticipate this to happen often. As is, such an error
					// would prevent the replica from picking up the latest Bloaty output, but this would
					// resolve itself as soon as the next Skia commit lands.
					//
					// If our metrics indicate that we're failing to process lots of messages, an alternative
					// is to Nack failed messages so as to retry them, and set up a dead-letter topic
					// (https://cloud.google.com/pubsub/docs/handling-failures) with a small
					// MaxDeliveryAttempts value in order to limit the number of retries.
					msg.Ack()
				} else {
					// TODO(lovisolo): Add a metrics counter.
					sklog.Infof("Done processing PubSub message: [bucket: %s, name: %s].\n", evt.Bucket, evt.Name)
					msg.Ack()
				}
			}); err != nil {
				// Receive returns a non-retryable error, so we log with Fatal to exit the program.
				sklog.Fatal(err)
			}
		}
	}()

	srv.loadTemplates()

	return srv, nil
}

// preloadBloatyFiles preloads the latest Bloaty outputs for each supported build artifact.
func (s *server) preloadBloatyFiles() error {
	// For now, this reads a single known Bloaty output file. Soon, we will replace this with a
	// directory structure of the form /<artifact name>/YYYY/MM/DD/<git hash>.{tsv/json}, where
	// <artifact name> is the name of the binary plus information about how it was built (e.g.
	// "dm-debug", "dm-release", etc.), the TSV file is the corresponding Bloaty output, and the JSON
	// file is a file with metadata such as the exact build parameters, Bloaty version and
	// command-line arguments, etc.
	//
	// TODO(lovisolo): Implement and test.

	contents, err := s.gcsClient.GetFileContents(context.Background(), "dm.tsv")
	if err != nil {
		return skerr.Wrap(err)
	}

	s.bloatyFile = string(contents[:])
	return nil
}

// handleFileUploadNotification is called when a new file is uplodaded to the GCS bucket.
func (s *server) handleFileUploadNotification(ctx context.Context, path string) error {
	// For now, this handler simply reads the contents of the uploaded file and prints them to stdout.
	// Eventually we will update the in-memory data structures with the contents and metadata of any
	// incoming files.
	//
	// TODO(lovisolo): Implement and test.
	gcsUrl := fmt.Sprintf("gs://%s/%s", gcsBucket, path)

	contents, err := s.gcsClient.GetFileContents(ctx, path)
	if err != nil {
		return skerr.Wrapf(err, "failed to get contents of %s", gcsUrl)
	}

	fmt.Printf("Contents of %s:\n%s\n", gcsUrl, string(contents[:]))
	return nil
}

// loadTemplates loads the HTML templaates to serve to the UI.
func (s *server) loadTemplates() {
	s.templates = template.Must(template.New("").Delims("{%", "%}").ParseGlob(
		filepath.Join(*baseapp.ResourcesDir, "*.html"),
	))
}

// sendJSONResponse sends a JSON representation of any data structure as an HTTP response. If the
// conversion to JSON has an error, the error is logged.
func sendJSONResponse(data interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// sendHTMLResponse renders the given template, passing it the current context's CSP nonce. If
// template rendering fails, it logs an error.
func (s *server) sendHTMLResponse(templateName string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := s.templates.ExecuteTemplate(w, templateName, map[string]string{
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (s *server) machinesPageHandler(w http.ResponseWriter, r *http.Request) {
	s.sendHTMLResponse("index.html", w, r)
}

func (s *server) bloatyHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(lovisolo): Parameterize this RPC and read the Bloaty output for the given artifact from
	//                 an in-memory cache.
	outputItems, err := bloaty.ParseTSVOutput(s.bloatyFile)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse dm.tsv.", http.StatusInternalServerError)
		return
	}

	res := rpc.BloatyRPCResponse{
		Rows: bloaty.GenTreeMapDataTableRows(outputItems),
	}
	sendJSONResponse(res, w)
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", s.machinesPageHandler).Methods("GET")
	r.HandleFunc("/rpc/bloaty/v1", s.bloatyHandler).Methods("GET")
}

// See baseapp.App.
func (s *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(new, []string{"codesize.skia.org"})
}
