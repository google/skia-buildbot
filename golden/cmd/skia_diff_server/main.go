package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/diffstore/failurestore/bolt_failurestore"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/bolt_and_fs_metricsstore"
	"google.golang.org/api/option"
	gstorage "google.golang.org/api/storage/v1"
	"google.golang.org/grpc"
)

// Command line flags.
var (
	cacheSize    = flag.Int("cache_size", 1, "Approximate cachesize used to cache images and diff metrics in GiB. This is just a way to limit caching. 0 means no caching at all. Use default for testing.")
	fsNamespace  = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
	fsProjectID  = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
	gsBucketName = flag.String("gs_bucket", "", "[required] Name of the Google Storage bucket that holds the uploaded images.")
	gsBaseDir    = flag.String("gs_basedir", diffstore.DefaultGCSImgDir, "String that represents the google storage directory/directories following the GS bucket")
	imageDir     = flag.String("image_dir", "/tmp/imagedir", "What directory to store test and diff images in.")
	imagePort    = flag.String("image_port", ":9001", "Address that serves image files via HTTP.")
	noCloudLog   = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
	grpcPort     = flag.String("grpc_port", ":9000", "gRPC service address (e.g., ':9000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

const (
	IMAGE_URL_PREFIX = "/img/"
)

func main() {

	// Parse the options, so we can configure logging.
	flag.Parse()

	// Set up the options.
	opts := []common.Opt{
		common.PrometheusOpt(promPort), // Enable Prometheus logging.
	}

	// Should we disable cloud logging.
	if !*noCloudLog {
		opts = append(opts, common.CloudLoggingOpt())
	}
	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, opts...)

	// Get the version of the repo.
	skiaversion.MustLogVersion()

	if *gsBucketName == "" {
		sklog.Fatalf("Must specify --gs_bucket")
	}

	// Get the client to be used to access GCS.
	ts, err := auth.NewDefaultTokenSource(*local, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Build the storage.Client client and wrap it around a gcs.GCSClient.
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Could not create storage client: %s.", err)
	}
	gcsClient := gcsclient.New(storageClient, *gsBucketName)

	// Set up ImageLoader failure store.
	fStore, err := bolt_failurestore.New(*imageDir)
	if err != nil {
		sklog.Fatalf("Could not create failure store: %s.", err)
	}
	sklog.Infof("ImageLoader failure store created at %s", *imageDir)

	// Auth note: the underlying firestore.NewClient looks at the GOOGLE_APPLICATION_CREDENTIALS env
	// variable, so we don't need to supply a token source.
	fsClient, err := firestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	// Build metrics store.
	// TODO(lovisolo): Replace with fs_metricsstore once we're confident enough that it works.
	mStore, err := bolt_and_fs_metricsstore.New(*imageDir, fsClient)
	if err != nil {
		sklog.Fatalf("Could not create metrics store: %s.", err)
	}

	memDiffStore, err := diffstore.NewMemDiffStore(gcsClient, *gsBaseDir, *cacheSize, mStore, fStore)
	if err != nil {
		sklog.Fatalf("Allocating DiffStore failed: %s", err)
	}

	// Create the server side instance of the DiffService.
	serverImpl := diffstore.NewDiffServiceServer(memDiffStore)
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(diffstore.MAX_MESSAGE_SIZE),
		grpc.MaxSendMsgSize(diffstore.MAX_MESSAGE_SIZE))
	diffstore.RegisterDiffServiceServer(grpcServer, serverImpl)

	// Set up the resource to serve the image files.
	imgHandler, err := memDiffStore.ImageHandler(IMAGE_URL_PREFIX)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}
	http.Handle(IMAGE_URL_PREFIX, imgHandler)
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	// Start the HTTP server.
	go func() {
		sklog.Infoln("Serving on http://127.0.0.1" + *imagePort)
		sklog.Fatal(http.ListenAndServe(*imagePort, nil))
	}()

	// Start the rRPC server.
	lis, err := net.Listen("tcp", *grpcPort)
	if err != nil {
		sklog.Fatalf("Error creating gRPC listener: %s", err)
	}
	sklog.Infof("Serving gRPC service on port %s", *grpcPort)
	sklog.Fatalf("Failure while serving gRPC service: %s", grpcServer.Serve(lis))
}
