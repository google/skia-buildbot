package poller

// Initializes and polls the various issue frameworks.

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/util"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
)

const (
	LivenessMetric = "skcq_be"

	SkiaRepoTmpl = "https://skia.googlesource.com/%s.git"
)

var (
	// Cache in-memory the list of supported projects + branches.
	// We are caching in-memory and not on disk because calculating if a project+branch is supported
	// is not much work.
	// TODO(rmistry): If support is removed for a project+branch then they will not be removed till the pod
	// is restarted. Bring up a separate go routine to periodically verify that everything in here is still
	// supported? Refresh the config at every tick here.
	ProjectsBranchesConfigCache = map[string]*config.SkCQCfg{}
)

// Start polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func Start(ctx context.Context, pollInterval time.Duration, cr codereview.CodeReview, supportedRepos []string, httpClient *http.Client) error {

	liveness := metrics2.NewLiveness(LivenessMetric)
	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		if !*baseapp.Local {
			// Ignore the passed-in context; this allows us to continue running even if the
			// context is canceled due to transient errors.
			ctx = context.Background()
		}

		fmt.Println("POLLING!")
		cls, err := cr.Search(ctx)
		if err != nil {
			sklog.Errorf("Error when searching for issues: %s", err)
		} else {
			// This should only be done if there are no errors.
			liveness.Reset()
		}

		for _, ci := range cls {
			// Debugging.
			fmt.Println("Debugging ALL matches")
			fmt.Println("-------")
			fmt.Println(ci.Issue)
			fmt.Println(ci.Project)
			fmt.Println(ci.Branch)
			fmt.Println("-------")

			// Only look at CLs that match the allowlist repos.
			if !util.In(ci.Project, supportedRepos) {
				sklog.Infof("%d in %s repo is not supported", ci.Issue, ci.Project)
				continue
			}

			repoBranch := fmt.Sprintf("%s/%s", ci.Project, ci.Branch)
			// Have we seen this project + branch before?
			if cachedCfg, ok := ProjectsBranchesConfigCache[repoBranch]; ok {
				sklog.Infof("WE HAVE SEEN %s BEFORE!!!", repoBranch)
				if cachedCfg == nil {
					sklog.Infof("%s DOES NOT HAVE A CONFIG SO CONTINUE!", repoBranch)
					continue
				}
			} else {
				sklog.Infof("WE HAVE NOT SEEN %s BEFORE!!!", repoBranch)
				// This is a repo we support.
				sklog.Infof("%d in %s repo is supported", ci.Issue, ci.Project)
				// Check to see if the branch has a XYZ file.
				gitilesRepo := gitiles.NewRepo(fmt.Sprintf(SkiaRepoTmpl, ci.Project), httpClient)

				cfg, err := config.ReadSkCQCfg(ctx, gitilesRepo, ci.Project, ci.Branch)
				if err != nil {
					// Either the config is missing or there was an error in reading it. Log the error
					// and move on.
					sklog.Errorf("Error reading skcq.cfg for %s", repoBranch)
					// Cache this repo+branch combination as being unsupported.
					ProjectsBranchesConfigCache[repoBranch] = nil
					continue
				}
				// Cache this repo+branch combination.
				ProjectsBranchesConfigCache[repoBranch] = cfg
			}
		}

		//

		fmt.Println("DONE")

	}, nil)

	return nil
}
