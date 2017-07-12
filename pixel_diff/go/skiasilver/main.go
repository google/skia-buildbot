package main

import (
  "flag"
  "html/template"
  "net/http"
  "os"
  "path/filepath"
  "runtime"
  "strings"

  "github.com/gorilla/mux"
  "go.skia.org/infra/go/auth"
  "go.skia.org/infra/go/common"
  "go.skia.org/infra/go/httputils"
  "go.skia.org/infra/go/login"
  "go.skia.org/infra/go/skiaversion"
  "go.skia.org/infra/go/sklog"
  "go.skia.org/infra/golden/go/diffstore"
  gstorage "google.golang.org/api/storage/v1"
  "google.golang.org/grpc"
)

// Command line flags.
var (
  appTitle           = flag.String("app_title", "Skia Silver", "Title of deployed app on front end")
  cacheSize          = flag.Int("cache_size", 1, "Approximate cachesize used to cache images and diff metrics in GiB. This is just a way to limit caching. 0 means no caching at all. Use default for testing.")
  diffServerGRPCPort = flag.String("diff_server_grpc", "", "The GRPC port of the diff server")
  diffServerAddr     = flag.String("diff_server_http", "", "The images serving address of the diff server")
  gsBucketNames      = flag.String("gs_buckets", "cluster-telemetry", "Comma-separated list of google storage bucket that hold uploaded images.")
  gsBaseDirs         = flag.String("gs_basedirs", "tasks/benchmark_runs", "Path of subdirectories after the GS bucket that lead to the uploaded images, not including the run directory")
  imageDir           = flag.String("image_dir", "/tmp/imagedir", "What directory to store test and diff images in.")
  local              = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
  noCloudLog         = flag.Bool("no_cloud_log", false, "Disables cloud logging. Primarily for running locally.")
  port               = flag.String("port", ":9999", "HTTP service address")
  promPort           = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
  resourcesDir       = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the directory relative to the source code files will be used.")
  serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

var (
  templates *template.Template
)

const (
  IMAGE_URL_PREFIX = "/img/"
)

func main() {
  defer common.LogPanic()

  // Parse the options, so we can configure logging.
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

  // Set the resource directory if it's empty
  if *resourcesDir == "" {
    _, filename, _, _ := runtime.Caller(0)
    *resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
    *resourcesDir += "/frontend"
  }

  // Get the client to be used to access GCS.
  client, err := auth.NewJWTServiceAccountClient("", *serviceAccountFile, nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
  if err != nil {
    sklog.Fatalf("Failed to authenticate service account: %s", err)
  }

  if (*diffServerGRPCPort != "") || (*diffServerAddr != "") {
    conn, err := grpc.Dial(*diffServerGRPCPort, grpc.WithInsecure())
    if err != nil {
      sklog.Fatalf("Unable to connect to GRPC service: %s", err)
    }

    diffStore, err = diffstore.NewNetDiffStore(conn, *diffServerAddr)
    if err != nil {
      sklog.Fatalf("Unable to initialize NetDiffStore: %s", err)
    }
  } else {
    diffStore, err = diffstore.NewMemDiffStore(client, *imageDir, strings.Split(*gsBucketNames, ","), *gsBaseDirs, *cacheSize, diffstore.GetCommonRunUrl, diffstore.GetCommonRunUrlImgName, diffstore.GetNoAndWithPatch)
    if err != nil {
      sklog.Fatalf("Allocating local DiffStore failed: %s", err)
    }
  }

  router := mux.NewRouter()

  // Set up the resource to serve the image files.
  imgHandler, err := diffStore.ImageHandler(IMAGE_URL_PREFIX)
  if err != nil {
    sklog.Fatalf("Unable to get image handler: %s", err)
  }
  router.PathPrefix(IMAGE_URL_PREFIX).Handler(imgHandler)

  router.PathPrefix("/res/").HandlerFunc(makeResourceHandler(*resourcesDir))

  router.HandleFunc("/", templateHandler("jobs.html"))
  router.HandleFunc("/jobs", templateHandler("jobs.html"))
  router.HandleFunc("/diff", templateHandler("results.html"))
  router.HandleFunc("/loginstatus/", login.StatusHandler)
  router.HandleFunc("/logout/", login.LogoutHandler)

  router.HandleFunc("/json/version", skiaversion.JsonHandler)

  router.HandleFunc("/json/jobs", jsonJobsHandler).Methods("GET")
  router.HandleFunc("/json/diff", jsonDiffHandler).Methods("GET")
  router.HandleFunc("/json/load", jsonLoadHandler).Methods("GET")
  router.HandleFunc("/json/sort", jsonSortHandler).Methods("GET")

  rootHandler := httputils.LoggingGzipRequestResponse(router)
  http.Handle("/", rootHandler)

  // Start the HTTP server.
  sklog.Infoln("Serving on http://127.0.0.1" + *port)
  sklog.Fatal(http.ListenAndServe(*port, nil))
}

func loadTemplates() {
  templates = template.Must(template.New("").ParseFiles(
    filepath.Join(*resourcesDir, "templates/jobs.html"),
    filepath.Join(*resourcesDir, "templates/results.html"),
    filepath.Join(*resourcesDir, "templates/header.html"),
  ))
}

func templateHandler(name string) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    if *local {
      loadTemplates()
    }
    appConfig := &struct {
      Title string `json:"title"`
    }{
    Title: *appTitle,
    }
    if err := templates.ExecuteTemplate(w, name, appConfig); err != nil {
      sklog.Errorln("Failed to expand template:", err)
    }
  }
}
