package main

import (
	"flag"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
	gstorage "google.golang.org/api/storage/v1"
	"google.golang.org/grpc"
)

// Command line flags.
var (
	cacheSize          = flag.Int("cache_size", 1, "Approximate cachesize used to cache images and diff metrics in GiB. This is just a way to limit caching. 0 means no caching at all. Use default for testing.")
	gsBucketNames      = flag.String("gs_buckets", "skia-infra-gm,chromium-skia-gm", "Comma-separated list of google storage bucket that hold uploaded images.")
	imageDir           = flag.String("image_dir", "/tmp/imagedir", "What directory to store test and diff images in.")
	imagePort          = flag.String("image_port", ":9001", "Address that serves image files via HTTP.")
	noCloudLog         = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
	port               = flag.String("port", ":9000", "gRPC service address (e.g., ':9000')")
	promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

const (
	IMAGE_URL_PREFIX = "/img/"
)

// diffStore handles all the diffing.
var diffStore diff.DiffStore = nil

func main() {
	defer common.LogPanic()
	var err error

	// Parse the options. So we can configure logging.
	flag.Parse()

	// Set up the logging options.
	logOpts := []common.Opt{
		common.PrometheusOpt(promPort),
	}

	// Should we disable cloud logging.
	if !*noCloudLog {
		logOpts = append(logOpts, common.CloudLoggingOpt())
	}
	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, logOpts...)

	// Get the version of the repo.
	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatalf("Unable to retrieve version: %s", err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	// Get the client to be used to access GCS.
	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountFile, nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	// Get the DiffStore that does the work loading and diffing images. .
	memDiffStore, err := diffstore.NewMemDiffStore(client, *imageDir, strings.Split(*gsBucketNames, ","), diffstore.DEFAULT_GCS_IMG_DIR_NAME, *cacheSize)
	if err != nil {
		sklog.Fatalf("Allocating DiffStore failed: %s", err)
	}

	// Create the server side instance of the DiffService.
	serverImpl := diffstore.NewDiffServiceServer(memDiffStore)
	grpcServer := grpc.NewServer()
	diffstore.RegisterDiffServiceServer(grpcServer, serverImpl)

	// Set up the resource to serve the image files.
	imgHandler, err := memDiffStore.ImageHandler(IMAGE_URL_PREFIX)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}
	http.Handle(IMAGE_URL_PREFIX, imgHandler)

	// Start the HTTP server.
	go func() {
		sklog.Infoln("Serving on http://127.0.0.1" + *port)
		sklog.Fatal(http.ListenAndServe(*port, nil))
	}()

	// Start the rRPC server.
	lis, err := net.Listen("tcp", *imagePort)
	if err != nil {
		sklog.Fatalf("Error creating gRPC listener: %s", err)
	}
	sklog.Infof("Serving gRPC service on port %s", *imagePort)
	sklog.Fatalf("Failure while serving gRPC service: %s", grpcServer.Serve(lis))
}
