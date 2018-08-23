package main

import (
	"context"
	"flag"
	"io"
	"net/http"
	"time"

	"github.com/flynn/json5"
	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
	"google.golang.org/api/option"
)

// flags
var (
	configFile = flag.String("config_file", "", "Configuration file to use.")
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port       = flag.String("port", ":8001", "HTTP service port (e.g., ':8000')")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	workdir    = flag.String("workdir", ".", "Directory to use for scratch work.")
)

func main() {
	common.InitWithMust(
		"google3-autoroll",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	defer common.Defer()

	skiaversion.MustLogVersion()

	var cfg roller.AutoRollerConfig
	if err := util.WithReadFile(*configFile, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&cfg)
	}); err != nil {
		sklog.Fatal(err)
	}

	ts, err := auth.NewDefaultTokenSource(*local)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := ds.InitWithOpt(common.PROJECT_ID, ds.AUTOROLL_INTERNAL_NS, option.WithTokenSource(ts)); err != nil {
		sklog.Fatal(err)
	}

	r := mux.NewRouter()
	if err := webhook.InitRequestSaltFromMetadata(metadata.WEBHOOK_REQUEST_SALT); err != nil {
		sklog.Fatal(err)
	}
	ctx := context.Background()
	arb, err := NewAutoRoller(ctx, *workdir, &cfg)
	if err != nil {
		sklog.Fatal(err)
	}
	arb.AddHandlers(r)
	arb.Start(ctx, time.Minute, time.Minute)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
