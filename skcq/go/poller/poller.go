package poller

// Initializes and polls the various issue frameworks.

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/verifiers"
)

const (
	LivenessMetric = "skcq_be"

	SkiaRepoTmpl = "https://skia.googlesource.com/%s.git"
)

var (
	// This is an optimization we can consider one day later.
	/*
		// Cache in-memory the list of supported projects + branches.
		// We are caching in-memory and not on disk because calculating if a project+branch is supported
		// is not much work.
		// TODO(rmistry): If support is removed for a project+branch then they will not be removed till the pod
		// is restarted. Bring up a separate go routine to periodically verify that everything in here is still
		// supported? Refresh the config at every tick here.
		// TODO(rmistry): Move this to the config package and have it be maintained there!
		ProjectsBranchesConfigCache = map[string]*config.SkCQCfg{}
	*/

	// Cache of CLs + Patchsets and their CQDetails.
	// This is to keep track of which CLs leave the CQ. Their verifiers are cleaned up when this happens.
	// This is also used to keep track of the start time of when the CQ started processing this CL. This
	// is useful to determine which CQ try jobs should be considered.

	// One issue - if CQ is restarted then this in-memory thing will be lost. and any cq job failures will not be considered
	// prior to this point?
	// is it better to persist startTime?
	ChangeAndPatchsetToCQDRecord = map[string]*CQDRecord{}
)

type CQDRecord struct {
	ci *gerrit.ChangeInfo

	// The time the CQ first looked at this change.
	// Uses unix epoch time.
	startTime int64

	changeVerifiers []verifiers.Verifier

	// Maybe. Though if we cache the config then we will not see any
	// live changes in skcq.cfg and tasks.json while the CL is running.
	// Simplest is not to cache this.
	// cfg *config.SkCQCfg
}

// Start polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func Start(ctx context.Context, pollInterval time.Duration, cr codereview.CodeReview, httpClient *http.Client, canModifyCfgsOnTheFly *allowed.AllowedFromChromeInfraAuth) error {

	liveness := metrics2.NewLiveness(LivenessMetric)
	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		if !*baseapp.Local {
			// Ignore the passed-in context; this allows us to continue running even if the
			// context is canceled due to transient errors.
			ctx = context.Background()
		}

		sklog.Info("----------------Poll--------------")
		cls, err := cr.Search(ctx)
		if err != nil {
			sklog.Errorf("Error when searching for issues: %s", err)
		} else {
			// This should only be done if there are no errors.
			liveness.Reset()
		}

		// Process the CLs now
		for _, ci := range cls {

			// TO MAKE IT EASIER FOR MY TESTING FOR NOW
			if ci.Project != "skiabot-playground" && ci.Project != "skiabot-test" {
				continue
			}

			repoBranch := fmt.Sprintf("%s/%s", ci.Project, ci.Branch)

			sklog.Infof("Processing %d for %s", ci.Issue, repoBranch)

			// Wasted majority of the time?? I don't know. Pass to verifiers!
			gitilesRepo := gitiles.NewRepo(fmt.Sprintf(SkiaRepoTmpl, ci.Project), httpClient)

			// The SkCQ cfg that will be used for this change.
			configReader, err := config.NewGitilesConfigReader(ctx, httpClient, ci, cr, canModifyCfgsOnTheFly)
			if err != nil {
				sklog.Errorf("[%d] Error when instantiating config reader: %s", ci.Issue, err)
				cr.RemoveFromCQ(ctx, ci, "Error when reading configs. Removing from CQ. Please ask Infra Gardener to investigate.")
				continue
			}
			skCQCfg, err := configReader.GetSkCQCfg(ctx)
			// TODO(rmistry): Also add a missing config error so it is easy to remove CLs for unsupported repos+branches.
			if err != nil {
				if _, ok := err.(*config.ConfigNotFoundError); ok {
					cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("%s. Removing from CQ. Please the config if this repo+branch requires CQ.", err.Error()))
					continue
				} else if _, ok := err.(*config.CannotModifyCfgsOnTheFlyError); ok {
					cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("CL owner %s does not have permission to modify %s", ci.Owner.Email, config.SkCQCfgPath))
					continue
				} else {
					cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("Error reading %s for %d. Removing from CQ. Please ask Infra Gardener to investigate.", config.SkCQCfgPath, ci.Issue))
					continue
				}
			}

			// At this point we have the config to use for the CL.
			fmt.Println("AT THIS POINT WE HAVE TH ECONFIG TO USE FOR THE CL")
			fmt.Printf("\n%+v\n", skCQCfg)

			// THESE VERIFIES SHOULD BE CACHED WITH THIS CL+PATCHSET COMBO. HOW DO YOU CLEANUP OLD CL+PATCHSETS? BY PUTING IN A MAP OR SOMETHING
			clVerifiers, err := verifiers.GetVerifiers(ctx, httpClient, skCQCfg, cr, ci, false /* isSubmittedTogetherChange */, gitilesRepo, configReader)
			if err != nil {
				sklog.Errorf("[%d] Error when getting verifiers: %s", ci.Issue, err)
				cr.RemoveFromCQ(ctx, ci, "Error when getting verifiers. Removing from CQ. Please ask Infra Gardener to investigate.")
				continue
			}
			// Log verifiers
			verifierNames := []string{}
			for _, clVerifier := range clVerifiers {
				verifierNames = append(verifierNames, clVerifier.Name())
			}
			sklog.Infof("[%d] uses verifiers: %s", ci.Issue, strings.Join(verifierNames, ", "))

			successMsgsFromVerifiers, waitMsgsFromVerifiers, rejectMsgsFromVerifiers := verifiers.RunVerifiers(ctx, ci, clVerifiers)
			if len(rejectMsgsFromVerifiers) > 0 {
				sklog.Infof("%d from %s has failed verifiers: %+v", ci.Issue, repoBranch, rejectMsgsFromVerifiers)
				cr.RemoveFromCQ(ctx, ci, strings.Join(rejectMsgsFromVerifiers, "\n"))
			} else if len(waitMsgsFromVerifiers) > 0 {
				sklog.Infof("%d from %s is waiting for verifiers: %+v", ci.Issue, repoBranch, waitMsgsFromVerifiers)
			} else {
				sklog.Infof("%d from %s successfully ran verifiers: %s", ci.Issue, repoBranch, strings.Join(successMsgsFromVerifiers, ", "))
				// All verifiers were successful. Either submit or say it is done.
				if cr.IsDryRun(ctx, ci) {
					// Say everything was succesful and we are done.
					cr.RemoveFromCQ(ctx, ci, "Dry run: This CL passed the SkCQ dry run.")
					// REMOVE FROM CACHE SOMEHOW.
				} else {
					if err := cr.Submit(ctx, ci); err != nil {
						sklog.Errorf("[%d] Error when submitted: %s", ci.Issue, err)
						cr.RemoveFromCQ(ctx, ci, "Error when submitting. Removing from CQ. Please ask Infra Gardener to investigate.")
					}
					fmt.Printf("\n\n%d SHOULD BE SUBMITTED!", ci.Issue)
				}
			}
		}

	}, nil)

	return nil
}
