package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	netpprof "net/http/pprof"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/diffstore/metricsstore/fs_metricsstore"
	"google.golang.org/api/option"
	gstorage "google.golang.org/api/storage/v1"
	"google.golang.org/grpc"
)

type diffServerConfig struct {
	config.Common
	// Approximate cachesize used to cache images and diff metrics in GiB. This is just a way to
	// limit caching.
	CacheSizeGB int `json:"cache_size_gb"`

	// The GCS prefix (directory) that holds the images that have been uploaded to Gold.
	GCSImageDir string `json:"gcs_image_dir"`

	// The port on which to run the GRPC service. The skiacorrectness binary will connect to this
	// server over this port, for example.
	GRPCPort string `json:"grpc_port"`

	// Address that serves image files via HTTP.
	ImagePort string `json:"image_port"`

	// Metrics service address (e.g., ':10110')
	PromPort string `json:"prom_port"`
}

const (
	imgURLPrefix = "/img/"
)

func main() {
	// Command line flags.
	var (
		commonInstanceConfig = flag.String("common_instance_config", "", "Path to the json5 file containing the configuration that needs to be the same across all services for a given instance.")
		thisConfig           = flag.String("config", "", "Path to the json5 file containing the configuration specific to diff server.")
		hang                 = flag.Bool("hang", false, "Stop and do nothing after reading the flags. Good for debugging containers.")
	)

	// Parse the flags, so we can load the configuration files.
	flag.Parse()

	if *hang {
		sklog.Info("Hanging")
		select {}
	}

	var dsc diffServerConfig
	if err := config.LoadFromJSON5(&dsc, commonInstanceConfig, thisConfig); err != nil {
		sklog.Fatalf("Reading config: %s", err)
	}
	sklog.Infof("Loaded config %#v", dsc)

	// Set up the options.
	opts := []common.Opt{
		common.PrometheusOpt(&dsc.PromPort), // Enable Prometheus logging.
	}

	_, appName := filepath.Split(os.Args[0])
	common.InitWithMust(appName, opts...)

	// Get the version of the repo.
	skiaversion.MustLogVersion()

	// Start the internal server on the internal port if requested.
	if dsc.DebugPort != "" {
		// Add the profiling endpoints to the internal router.
		internalRouter := mux.NewRouter()

		// Register pprof handlers
		internalRouter.HandleFunc("/debug/pprof/", netpprof.Index)
		internalRouter.HandleFunc("/debug/pprof/symbol", netpprof.Symbol)
		internalRouter.HandleFunc("/debug/pprof/profile", netpprof.Profile)
		internalRouter.HandleFunc("/debug/pprof/{profile}", netpprof.Index)

		go func() {
			sklog.Infof("Internal server on http://127.0.0.1" + dsc.DebugPort)
			sklog.Fatal(http.ListenAndServe(dsc.DebugPort, internalRouter))
		}()
	}

	// Get the client to be used to access GCS.
	ts, err := auth.NewDefaultTokenSource(dsc.Local, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Build the storage.Client client and wrap it around a gcs.GCSClient.
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Could not create storage client: %s.", err)
	}
	gcsClient := gcsclient.New(storageClient, dsc.GCSBucket)

	// Auth note: the underlying firestore.NewClient looks at the GOOGLE_APPLICATION_CREDENTIALS env
	// variable, so we don't need to supply a token source.
	fsClient, err := firestore.NewClient(context.Background(), dsc.FirestoreProjectID, "gold", dsc.FirestoreNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	// Build metrics store.
	mStore := fs_metricsstore.New(fsClient)

	memDiffStore, err := diffstore.NewMemDiffStore(gcsClient, dsc.GCSImageDir, dsc.CacheSizeGB, mStore)
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
	imgHandler, err := memDiffStore.ImageHandler(imgURLPrefix)
	if err != nil {
		sklog.Fatalf("Unable to get image handler: %s", err)
	}
	http.Handle(imgURLPrefix, imgHandler)
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	// Start the HTTP server.
	go func() {
		sklog.Info("Serving on http://127.0.0.1" + dsc.ImagePort)
		sklog.Fatal(http.ListenAndServe(dsc.ImagePort, nil))
	}()

	// Start the rRPC server.
	lis, err := net.Listen("tcp", dsc.GRPCPort)
	if err != nil {
		sklog.Fatalf("Error creating gRPC listener: %s", err)
	}
	sklog.Infof("Serving gRPC service on port %s", dsc.GRPCPort)
	sklog.Fatalf("Failure while serving gRPC service: %s", grpcServer.Serve(lis))
}
