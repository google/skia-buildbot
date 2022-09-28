package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/codesize/go/bloaty"
	"go.skia.org/infra/codesize/go/codesizeserver/rpc"
	"go.skia.org/infra/codesize/go/store"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	gcpProjectName = "skia-public"
	gcsBucket      = "skia-codesize"
	pubSubTopic    = "skia-codesize-files"

	// numMostRecentBinaries is the number of most recent binaries to show on the index page (grouped
	// by changelist or patchset).
	numMostRecentBinaries = 20

	// Preload the last 10 days (arbitrarily chosen) worth of data. Past that is probably not
	// relevant to most developers, and loading too much data can make the server take a long
	// time to load.
	daysToPreload = 10

	// Some sections (e.g. .text) have a lot of children. This groups them up beyond a certain
	// point to help the treemap not be too hard to read or slow to render.
	maxChildrenPerParent = 200
)

var exponentialBackoffSettings = &backoff.ExponentialBackOff{
	InitialInterval:     5 * time.Second,
	RandomizationFactor: 0.5,
	Multiplier:          2,
	MaxInterval:         time.Minute,
	MaxElapsedTime:      5 * time.Minute,
	Clock:               backoff.SystemClock,
}

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
	store     store.Store
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	ctx := context.Background()
	srv := &server{}

	// Set up GCS client.
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadWrite)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get token source")
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create storage client")
	}
	srv.gcsClient = gcsclient.New(storageClient, gcsBucket)

	// Set up Store.
	s := store.New(func(ctx context.Context, path string) ([]byte, error) {
		sklog.Infof("Downloading %s from GCS", path)

		var contents []byte
		downloadFunc := func() error {
			contents, err = srv.gcsClient.GetFileContents(ctx, path)
			return err
		}
		if err := backoff.Retry(downloadFunc, exponentialBackoffSettings); err != nil {
			return nil, skerr.Wrapf(err, "even with exponential backoff, could not download %s", path)
		}

		return contents, nil
	})
	srv.store = s

	// Preload the latest Bloaty outputs.
	if err := srv.preloadBloatyFiles(ctx); err != nil {
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
					// Some of these errors might be retriable, such as transient network errors while
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
func (s *server) preloadBloatyFiles(ctx context.Context) error {
	// We'll filter out anything that isn't under a YYYY/MM/DD directory and doesn't end in ".tsv".
	// This excludes debug files that we occasionally upload to GCS.
	bloatyOutputFilePattern := regexp.MustCompile(`^[0-9]{4}/[0-9]{2}/[0-9]{2}/.*\.tsv$`)

	n := now.Now(ctx).UTC()
	dirs := fileutil.GetHourlyDirs("", n.Add(-daysToPreload*24*time.Hour), n)
	if *baseapp.Local {
		dirs = fileutil.GetHourlyDirs("", n.Add(-8*time.Hour), n)
	}
	sklog.Debugf("Preloading data, starting in folder %s", dirs[0])
	for _, dir := range dirs {
		err := s.gcsClient.AllFilesInDirectory(ctx, dir, func(item *storage.ObjectAttrs) error {
			if !bloatyOutputFilePattern.MatchString(item.Name) {
				return nil
			}
			if err := s.store.Index(ctx, item.Name); err != nil {
				// If this happens often (e.g. because we're hitting a GCS QPS limit) we can do a combination
				// of limiting our QPS rate and only fetching the most recent files.
				//
				// As is, this is probably fine. At worst, the app will crash and Kubernetes will retry
				// creating the pod, entering a crash loop if we're still over the QPS quota. Exponential
				// backoff would be a potential mitigation.
				sklog.Fatalf("Error while preloading %s: %s\n", item.Name, err)
			}
			return nil
		})
		if err != nil {
			return skerr.Wrap(err)
		}
	}

	return nil
}

// handleFileUploadNotification is called when a new file is uploaded to the GCS bucket.
func (s *server) handleFileUploadNotification(ctx context.Context, path string) error {
	sklog.Infof("Received file upload PubSub message: %s", path)
	if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".diff.txt") {
		sklog.Infof("Ignoring %s because we index .json and .diff.txt files when we see a corresponding .tsv file", path)
		return nil
	}
	if err := s.store.Index(ctx, path); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// loadTemplates loads the HTML templates to serve to the UI.
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

func (s *server) indexPageHandler(w http.ResponseWriter, r *http.Request) {
	s.sendHTMLResponse("index.html", w, r)
}

func (s *server) binaryPageHandler(w http.ResponseWriter, r *http.Request) {
	s.sendHTMLResponse("binary.html", w, r)
}

func (s *server) binaryDiffPageHandler(w http.ResponseWriter, r *http.Request) {
	s.sendHTMLResponse("binary_diff.html", w, r)
}

func (s *server) binaryRPCHandler(w http.ResponseWriter, r *http.Request) {
	req := rpc.BinaryRPCRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to parse request", http.StatusBadRequest)
		return
	}

	binary, ok := s.store.GetBinary(req.CommitOrPatchset, req.BinaryName, req.CompileTaskName)
	if !ok {
		httputils.ReportError(w, nil, "Binary not found in Store", http.StatusNotFound)
		return
	}

	bytes, err := s.store.GetBloatyOutputFileContents(r.Context(), binary)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve Bloaty output file", http.StatusInternalServerError)
		return
	}

	outputItems, err := bloaty.ParseTSVOutput(string(bytes))
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse Bloaty output file.", http.StatusInternalServerError)
		return
	}

	res := rpc.BinaryRPCResponse{
		Metadata: binary.Metadata,
		Rows:     bloaty.GenTreeMapDataTableRows(outputItems, maxChildrenPerParent),
	}
	sendJSONResponse(res, w)
}

func (s *server) binarySizeDiffRPCHandler(w http.ResponseWriter, r *http.Request) {
	req := rpc.BinarySizeDiffRPCRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to parse request", http.StatusBadRequest)
		return
	}

	binary, ok := s.store.GetBinary(req.CommitOrPatchset, req.BinaryName, req.CompileTaskName)
	if !ok {
		httputils.ReportError(w, nil, "Binary not found in Store", http.StatusNotFound)
		return
	}

	bytes, err := s.store.GetBloatySizeDiffOutputFileContents(r.Context(), binary)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve Bloaty output file", http.StatusInternalServerError)
		return
	}

	res := rpc.BinarySizeDiffRPCResponse{
		Metadata: binary.Metadata,
		RawDiff:  string(bytes),
	}
	sendJSONResponse(res, w)
}

func (s *server) mostRecentBinariesRPCHandler(w http.ResponseWriter, r *http.Request) {
	binaries := s.store.GetMostRecentBinaries(numMostRecentBinaries)
	res := rpc.MostRecentBinariesRPCResponse{
		Binaries: binaries,
	}
	sendJSONResponse(res, w)
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", s.indexPageHandler).Methods("GET")
	r.HandleFunc("/binary", s.binaryPageHandler).Methods("GET")
	r.HandleFunc("/binary_diff", s.binaryDiffPageHandler).Methods("GET")
	r.HandleFunc("/rpc/binary/v1", s.binaryRPCHandler).Methods("POST")
	r.HandleFunc("/rpc/binary_size_diff/v1", s.binarySizeDiffRPCHandler).Methods("POST")
	r.HandleFunc("/rpc/most_recent_binaries/v1", s.mostRecentBinariesRPCHandler).Methods("GET")
}

// See baseapp.App.
func (s *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(new, []string{"codesize.skia.org"})
}
