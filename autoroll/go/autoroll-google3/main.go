package main

import (
	"context"
	"encoding/base64"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config/db"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/prototext"
)

// flags
var (
	configContents    = flag.String("config", "", "Base 64 encoded configuration in JSON format, mutually exclusive with --config_file.")
	configFile        = flag.String("config_file", "", "Configuration file to use.")
	firestoreInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	webhookSalt       = flag.String("webhook_request_salt", "", "Path to a file containing webhook request salt.")
)

func main() {
	common.InitWithMust(
		"google3-autoroll",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	defer common.Defer()

	if *webhookSalt == "" {
		sklog.Fatal("--webhook_request_salt is required.")
	}

	// Decode the config.
	if (*configContents == "" && *configFile == "") || (*configContents != "" && *configFile != "") {
		sklog.Fatal("Exactly one of --config or --config_file is required.")
	}
	var configBytes []byte
	var err error
	if *configContents != "" {
		configBytes, err = base64.StdEncoding.DecodeString(*configContents)
	} else {
		err = util.WithReadFile(*configFile, func(f io.Reader) error {
			configBytes, err = ioutil.ReadAll(f)
			return err
		})
	}
	if err != nil {
		sklog.Fatal(err)
	}
	var cfg config.Config
	if err := prototext.Unmarshal(configBytes, &cfg); err != nil {
		sklog.Fatal(err)
	}

	ctx := context.Background()

	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, auth.ScopeGerrit, datastore.ScopeDatastore)
	if err != nil {
		sklog.Fatal(err)
	}
	const namespace = ds.AUTOROLL_INTERNAL_NS
	if err := ds.InitWithOpt(common.PROJECT_ID, namespace, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	if !*local {
		// Update the roller config in the DB.
		configDB, err := db.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, namespace, *firestoreInstance, ts)
		if err != nil {
			sklog.Fatal(err)
		}
		if err := configDB.Put(ctx, cfg.RollerName, &cfg); err != nil {
			sklog.Fatal(err)
		}
	}

	r := mux.NewRouter()
	if err := webhook.InitRequestSaltFromFile(*webhookSalt); err != nil {
		sklog.Fatal(err)
	}
	arb, err := NewAutoRoller(ctx, &cfg, client, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	arb.AddHandlers(r)
	arb.Start(ctx, time.Minute, time.Minute)
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
