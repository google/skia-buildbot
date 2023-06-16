package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config/db"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/webhook"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// flags
var (
	rollerName        = flag.String("roller_name", "", "Name of the roller.")
	childBranch       = flag.String("child_branch", "", "Git branch of the child.")
	childRepo         = flag.String("child_repo", "", "Git repo URL of the child.")
	childDisplayName  = flag.String("child_display_name", "", "Display name of the child.")
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
	)
	defer common.Defer()

	if *webhookSalt == "" {
		sklog.Fatal("--webhook_request_salt is required.")
	}

	// Create the config. A lot of functionality requires an actual config
	// struct, so we fake most of it here.
	cfg := config.Config{
		RollerName:        *rollerName,
		ChildDisplayName:  *childDisplayName,
		ParentDisplayName: "Google3",
		ParentWaterfall:   "https://goto.google.com/skia-testing-status",
		OwnerPrimary:      "fake",
		OwnerSecondary:    "fake",
		Contacts:          []string{"fake"},
		ServiceAccount:    "fake",
		IsInternal:        true,
		Reviewer:          []string{"fake"},
		CommitMsg: &config.CommitMsgConfig{
			BuiltIn: config.CommitMsgConfig_DEFAULT,
		},
		CodeReview: &config.Config_Google3{
			Google3: &config.Google3Config{},
		},
		Kubernetes: &config.KubernetesConfig{
			Cpu:    "fake",
			Memory: "fake",
			Disk:   "fake",
			Image:  "fake",
		},
		RepoManager: &config.Config_Google3RepoManager{
			Google3RepoManager: &config.Google3RepoManagerConfig{
				ChildBranch: *childBranch,
				ChildRepo:   *childRepo,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
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

	r := chi.NewRouter()
	if err := webhook.InitRequestSaltFromFile(*webhookSalt); err != nil {
		sklog.Fatal(err)
	}
	statusDB, err := status.NewDB(ctx, firestore.FIRESTORE_PROJECT, namespace, *firestoreInstance, ts)
	if err != nil {
		sklog.Fatalf("Failed to create status DB: %s", err)
	}
	arb, err := NewAutoRoller(ctx, &cfg, client, statusDB)
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
