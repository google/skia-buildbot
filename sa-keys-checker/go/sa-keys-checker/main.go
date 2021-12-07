package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	admin "cloud.google.com/go/iam/admin/apiv1"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	adminpb "google.golang.org/genproto/googleapis/iam/admin/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	//Metric names.
	saKeyExpirationMetric = "sa_key_expiration_s"
	livenessMetric        = "sa_keys_checker"
)

func main() {
	// Flags.
	cloudProjects := common.NewMultiStringFlag("cloud_project", nil, "Cloud projects in which this will scan all service accounts for expiring and expired keys.")
	promPort := flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	local := flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	pollPeriod := flag.Duration("poll_period", 5*time.Minute, "How often to check for expired service account keys.")

	common.InitWithMust("sa_keys_checker", common.PrometheusOpt(promPort), common.MetricsLoggingOpt())
	defer sklog.Flush()
	ctx := context.Background()

	if len(*cloudProjects) == 0 {
		sklog.Fatal("Must specify atleast one --cloud_project")
	}

	ts, err := auth.NewDefaultTokenSource(*local, auth.ScopeUserinfoEmail, auth.ScopeAllCloudAPIs)
	if err != nil {
		sklog.Fatalf("Could not create token source: %s", err)
	}

	// Pre-populate a map of cloud projects to the clients to use for them.
	clients := map[string]*admin.IamClient{}
	for _, p := range *cloudProjects {
		creds := &google.Credentials{
			ProjectID:   p,
			TokenSource: ts,
		}
		c, err := admin.NewIamClient(ctx, option.WithCredentials(creds))
		if err != nil {
			sklog.Fatalf("Failed to create IAM client: %s", err)
		}
		clients[p] = c
	}

	liveness := metrics2.NewLiveness(livenessMetric)
	oldMetrics := map[metrics2.Float64Metric]struct{}{}
	go util.RepeatCtx(ctx, *pollPeriod, func(ctx context.Context) {
		newMetrics, err := performChecks(ctx, clients, oldMetrics)
		if err != nil {
			sklog.Errorf("Error when checking for service account keys: %s", err)
		} else {
			liveness.Reset()
			oldMetrics = newMetrics
		}
	})

	select {}
}

func performChecks(ctx context.Context, clients map[string]*admin.IamClient, oldMetrics map[metrics2.Float64Metric]struct{}) (map[metrics2.Float64Metric]struct{}, error) {
	sklog.Info("-----------New round of checking service account keys expiration-----------")
	newMetrics := map[metrics2.Float64Metric]struct{}{}

	// * For each specified cloud project do the following:
	//   * Get list of all service accounts in that project and loop through them:
	//     * Get list of all keys that exist for those service accounts and loop through them:
	//       * Discard all auto-generated short lived keys.
	//       * Create and update a metric for the manually created keys.
	for p, c := range clients {
		saEmails := []string{}
		saIterator := c.ListServiceAccounts(ctx, &adminpb.ListServiceAccountsRequest{Name: fmt.Sprintf("projects/%s", p)})

		for {
			sa, err := saIterator.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				return nil, skerr.Wrap(err)
			}
			saEmails = append(saEmails, sa.Email)
		}

		for _, sa := range saEmails {
			serviceAccountPath := admin.IamServiceAccountPath(p, sa)
			resp, err := c.ListServiceAccountKeys(ctx, &adminpb.ListServiceAccountKeysRequest{
				Name: serviceAccountPath,
			})
			if err != nil {
				return nil, fmt.Errorf("Failed to list service account keys of %s: %s", sa, err)
			}
			for _, k := range resp.GetKeys() {
				processKey(k, newMetrics, sa, p)
			}
		}
	}
	sklog.Infof("Updated %d metrics", len(newMetrics))

	// Delete no longer used metrics.
	deleteUnusedMetrics(oldMetrics, newMetrics)

	return newMetrics, nil
}

// deleteUnusedMetrics finds all metrics that are in oldMetrics but not
// newMetrics and deletes them.
func deleteUnusedMetrics(oldMetrics, newMetrics map[metrics2.Float64Metric]struct{}) {
	deleteCount := 0
	for m := range oldMetrics {
		if _, ok := newMetrics[m]; !ok {
			if err := m.Delete(); err != nil {
				sklog.Errorf("Failed to delete metric: %s", err)
				// Add the metric to newMetrics so that we'll
				// have the chance to delete it again on the
				// next cycle.
				newMetrics[m] = struct{}{}
			} else {
				deleteCount += 1
			}
		}
	}
	sklog.Infof("Deleted %d old metrics", deleteCount)
}

// processKey determines whether the provided service account key is
// manually created. If it is then it updates a metric for that key.
func processKey(key *adminpb.ServiceAccountKey, metrics map[metrics2.Float64Metric]struct{}, sa, project string) {
	validAfter := time.Unix(key.GetValidAfterTime().Seconds, 0)
	validBefore := time.Unix(key.GetValidBeforeTime().Seconds, 0)
	duration := validBefore.Sub(validAfter)
	if duration > 21*24*time.Hour {
		// GCE seems to auto-generate keys with short
		// expirations. These do not show up on the UI.
		// Don't mess with them and only look at the longer
		// lived keys which we've created.

		duration = validBefore.Sub(time.Now())
		// Convert the long key name that looks like
		// "projects/skia-public/serviceAccounts/android-autoroll@skia-public.iam.gserviceaccount.com/keys/abc"
		// into the much more readable "abc".
		keyTokens := strings.Split(key.GetName(), "/")
		shortKeyName := keyTokens[len(keyTokens)-1]

		tags := map[string]string{
			"sa":      sa,
			"key":     shortKeyName,
			"project": project,
		}
		metric := metrics2.GetFloat64Metric(saKeyExpirationMetric, tags)
		metric.Update(duration.Seconds())
		metrics[metric] = struct{}{}
	}
}
