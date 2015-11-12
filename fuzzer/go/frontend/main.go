package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/fuzzer/go/fuzz"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil
	// detailsTemplate is used for /details, which actually displays the stacktraces and fuzzes.
	detailsTemplate *template.Template = nil
)

// Command line flags.
var (
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":80", "HTTP service port (e.g., ':8002')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")

	authWhiteList = flag.String("auth_whitelist", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
	redirectURL   = flag.String("redirect_url", "https://fuzzer.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")
)

func Init() {
	reloadTemplates()
}

func reloadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	detailsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/details.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func main() {
	defer common.LogPanic()
	// Calls flag.Parse()
	common.InitWithMetrics("fuzzer", graphiteServer)

	Init()

	setupOAuth()

	runServer()
}

func setupOAuth() {
	var cookieSalt = "notverysecret"
	// This clientID and clientSecret are only used for setting up a local server.
	// Production id and secrets are in metadata and will be loaded from there.
	var clientID = "31977622648-ubjke2f3staq6ouas64r31h8f8tcbiqp.apps.googleusercontent.com"
	var clientSecret = "rK-kRY71CXmcg0v9I9KIgWci"
	var useRedirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
		useRedirectURL = *redirectURL
	}
	login.Init(clientID, clientSecret, useRedirectURL, cookieSalt, login.DEFAULT_SCOPE, *authWhiteList, *local)
}

func runServer() {
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))

	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/details", detailHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/json/fuzz-list", fuzzListHandler)
	r.HandleFunc("/json/details", detailsHandler)

	rootHandler := login.ForceAuth(util.LoggingGzipRequestResponse(r), OAUTH2_CALLBACK_PATH)

	http.Handle("/", rootHandler)
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}

	w.Header().Set("Content-Type", "text/html")

	if err := indexTemplate.Execute(w, nil); err != nil {
		glog.Errorf("Failed to expand template: %v", err)
	}
}

func detailHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}

	w.Header().Set("Content-Type", "text/html")

	if err := detailsTemplate.Execute(w, nil); err != nil {
		glog.Errorf("Failed to expand template: %v", err)
	}
}

func fuzzListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// TODO(kjlubick): fill this in with real data.
	mockFuzzes := fuzz.FuzzReport{
		{
			"foo.h", 30, 0, []fuzz.FuzzReportFunction{
				{
					"frizzle()", 18, 0, []fuzz.FuzzReportLineNumber{
						{
							64, 17, 0, []fuzz.FuzzReportBinary{},
						},
						{
							69, 1, 0, []fuzz.FuzzReportBinary{},
						},
					},
				}, {
					"zizzle()", 12, 0, []fuzz.FuzzReportLineNumber{
						{
							123, 12, 0, []fuzz.FuzzReportBinary{},
						},
					},
				},
			},
		}, {
			"bar.h", 15, 3, []fuzz.FuzzReportFunction{
				{
					"frizzle()", 15, 3, []fuzz.FuzzReportLineNumber{
						{
							566, 15, 2, []fuzz.FuzzReportBinary{},
						},
						{
							568, 0, 1, []fuzz.FuzzReportBinary{},
						},
					},
				},
			},
		},
	}

	if err := json.NewEncoder(w).Encode(mockFuzzes); err != nil {
		glog.Errorf("Failed to write or encode output: %v", err)
		return
	}
}

func detailsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// TODO(kjlubick): fill this in with real data.
	mockBinaryFuzz := fuzz.FuzzReportBinary{
		DebugStackTrace: fuzz.StackTrace{
			Frames: []fuzz.StackTraceFrame{
				fuzz.BasicStackFrame("src/core/", "SkReadBuffer.cpp", 344),
				fuzz.BasicStackFrame("src/core/", "SkReadBuffer.h", 130),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 498),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 424),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
				fuzz.BasicStackFrame("src/core/", "SkPicture.cpp", 153),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 392),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
				fuzz.BasicStackFrame("src/core/", "SkPicture.cpp", 153),
				fuzz.BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 41),
			},
		},
		ReleaseStackTrace: fuzz.StackTrace{
			Frames: []fuzz.StackTraceFrame{
				fuzz.BasicStackFrame("src/core/", "SkReadBuffer.h", 130),
				fuzz.BasicStackFrame("src/core/", "SkReadBuffer.h", 136),
				fuzz.BasicStackFrame("src/core/", "SkPaint.cpp", 1971),
				fuzz.BasicStackFrame("src/core/", "SkReadBuffer.h", 126),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 498),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 424),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 553),
				fuzz.BasicStackFrame("src/core/", "SkPicture.cpp", 153),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 392),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
				fuzz.BasicStackFrame("src/core/", "SkPictureData.cpp", 553),
				fuzz.BasicStackFrame("src/core/", "SkPicture.cpp", 153),
				fuzz.BasicStackFrame("src/core/", "SkPicture.cpp", 142),
				fuzz.BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 41),
				fuzz.BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 71),
			},
		},
		HumanReadableFlags: []string{"DebugDumped", "DebugAssertion", "ReleaseTimedOut"},
		BadBinaryName:      "badbeef",
		BinaryType:         "skp",
	}

	mockFuzzes := fuzz.FuzzReportFile{
		FileName:    "foo.h",
		BinaryCount: 9,
		ApiCount:    0,
		Functions: []fuzz.FuzzReportFunction{
			{
				"frizzle()", 9, 0, []fuzz.FuzzReportLineNumber{
					{
						64, 8, 0, []fuzz.FuzzReportBinary{mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz, mockBinaryFuzz},
					},
					{
						69, 1, 0, []fuzz.FuzzReportBinary{mockBinaryFuzz},
					},
				},
			},
		},
	}

	if err := json.NewEncoder(w).Encode(mockFuzzes); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}
