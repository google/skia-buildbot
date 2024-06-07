// This zone-apply application checks out the zone files from the buildbot repo
// and applies them using the `gcloud` command line application every five
// minutes.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2/google"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// These could be changed to flags if needed.
const (
	refreshConfigsDuration = 5 * time.Minute
	repo                   = "https://skia.googlesource.com/buildbot"
)

// flags
var (
	promPort = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
	dryRun   = flag.Bool("dryrun", false, "Set to true to dry run, i.e. only get the files w/o executing the apply command.")
)

// Zone describes a single zone file and it's associated information.
type Zone struct {
	// Path to the zone file in the repo, for example: "skfe/skia.org.org".
	filename string

	// GCP project that owns the DNS records, for example: "skia-pubic".
	project string

	// GCP Zone name, for example: "skia-org".
	zoneName string
}

var (
	zoneFiles = []Zone{
		{
			filename: "skfe/skia.org.zone",
			project:  "skia-public",
			zoneName: "skia-org",
		},
		{
			filename: "skfe/luci.app.zone",
			project:  "skia-public",
			zoneName: "luci-app",
		},
	}
)

// server is the state of the server.
type server struct {
	gitilesRepo              gitiles.GitilesRepo
	zoneApplyRefreshLiveness metrics2.Liveness
	zoneHasError             []metrics2.BoolMetric
}

func main() {
	common.InitWithMust("zone-apply",
		common.PrometheusOpt(promPort),
	)
	ctx := context.Background()

	client, err := google.DefaultClient(ctx, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatalf("Creating authenticated HTTP client: %s", err)
	}
	gitilesRepo := gitiles.NewRepo(repo, client)

	srv := &server{
		gitilesRepo:              gitilesRepo,
		zoneApplyRefreshLiveness: metrics2.NewLiveness("zone_apply_refresh"),
		zoneHasError:             []metrics2.BoolMetric{},
	}

	for _, zone := range zoneFiles {
		srv.zoneHasError = append(srv.zoneHasError, metrics2.GetBoolMetric("zone_has_error", map[string]string{"filename": zone.filename}))
	}

	if err := srv.applyZones(ctx); err != nil {
		sklog.Fatal(err)
	}

	srv.periodicallyApplyZoneFiles(ctx)
}

// periodicallyApplyZoneFiles does not return and continuously checks out and
// applies the latest zone files.
func (srv *server) periodicallyApplyZoneFiles(ctx context.Context) {
	for range time.Tick(refreshConfigsDuration) {
		if err := srv.applyZones(ctx); err != nil {
			sklog.Errorf("Failed to refresh configs: %s", err)
		}
	}
}

func (srv *server) checkoutAndApplyZoneFile(ctx context.Context, zone Zone) error {
	// Create a temp file to write the zone file to, since we need a file on
	// disk for the `gcloud` command to work with.
	tmpFile, err := os.CreateTemp("", "zone-apply")
	if err != nil {
		return skerr.Wrapf(err, "creating temp file to store zone file")
	}
	if err := tmpFile.Close(); err != nil {
		return skerr.Wrapf(err, "closing temp file")
	}
	defer util.Remove(tmpFile.Name())

	if err := srv.gitilesRepo.DownloadFile(ctx, zone.filename, tmpFile.Name()); err != nil {
		return skerr.Wrapf(err, "downloading zone file %q from gitiles", zone.filename)
	}

	return applyZoneFile(ctx, zone, tmpFile.Name())
}

func applyZoneFile(ctx context.Context, zone Zone, filename string) error {

	// Now run the gcloud command to apply the zone file, for example:
	//
	//     gcloud dns record-sets import --project skia-public --delete-all-existing --zone skia-org --zone-file-format skia.org.zone
	//
	// It is safe to reapply the files because the gcloud cli determines if any
	// records need to be updated, and does nothing if they all already exist.

	gcloudArgs := fmt.Sprintf("dns record-sets import --project %s --delete-all-existing --zone %s --zone-file-format %s", zone.project, zone.zoneName, filename)
	if *dryRun {
		b, err := os.ReadFile(filename)
		if err == nil {
			fmt.Println(string(b))
		}
		sklog.Infof("DRYRUN - Not executing: gcloud %s", gcloudArgs)
		return nil
	}

	gcloupArgsSplit := strings.Split(gcloudArgs, " ")
	output, err := executil.CommandContext(ctx, "gcloud", gcloupArgsSplit...).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "executing: %q got: %q", gcloudArgs, string(output))
	}
	sklog.Info(string(output))

	return nil
}

func (srv *server) applyZones(ctx context.Context) error {
	srv.zoneApplyRefreshLiveness.Reset()

	for i, zone := range zoneFiles {
		err := srv.checkoutAndApplyZoneFile(ctx, zone)
		srv.zoneHasError[i].Update(err != nil)
		if err != nil {
			sklog.Error(err)
		}
	}

	return nil
}
