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
	// Cache in-memory the list of supported projects + branches.
	// We are caching in-memory and not on disk because calculating if a project+branch is supported
	// is not much work.
	// TODO(rmistry): If support is removed for a project+branch then they will not be removed till the pod
	// is restarted. Bring up a separate go routine to periodically verify that everything in here is still
	// supported? Refresh the config at every tick here.
	ProjectsBranchesConfigCache = map[string]*config.SkCQCfg{}

	// TODO(rmistry): THIS THIS
	// Cache of CLs + Patchsets and their configs.
	// THIS SHOULD BE A MAP OF MAP I THINK. TO HELP CLEANUP OLD CLS!
	ChangeAndPatchsetToConfig = map[string]*config.SkCQCfg{}
)

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

		for _, ci := range cls {

			// TO MAKE IT EASIER FOR MY TESTING FOR NOW
			if ci.Project != "skiabot-playground" {
				continue
			}

			repoBranch := fmt.Sprintf("%s/%s", ci.Project, ci.Branch)

			sklog.Infof("Processing %d for %s", ci.Issue, repoBranch)

			// Wasted majority of the time?? I don't know.
			gitilesRepo := gitiles.NewRepo(fmt.Sprintf(SkiaRepoTmpl, ci.Project), httpClient)

			// The SkCQ cfg that will be used for this change.
			var cfg *config.SkCQCfg

			// Have we seen this project + branch before?
			if cachedCfg, ok := ProjectsBranchesConfigCache[repoBranch]; ok {
				sklog.Infof("WE HAVE SEEN %s BEFORE!!!", repoBranch)
				if cachedCfg == nil {
					sklog.Infof("%s DOES NOT HAVE A CONFIG SO CONTINUE!", repoBranch)

					// (this will be making a lot of requests then but maybe that's ok).
					patchsetIDs := ci.GetPatchsetIDs()
					latestPatchsetID := patchsetIDs[len(patchsetIDs)-1]
					gerritChangeRef := fmt.Sprintf("%s%d/%d/%d", gerrit.ChangeRefPrefix, ci.Issue%100, ci.Issue, latestPatchsetID)
					cfg, err = config.ReadSkCQCfg(ctx, gitilesRepo, ci.Project, gerritChangeRef)
					if err != nil {
						// The config does not exist. Move on.
						continue
					} else {
						if !canModifyCfgsOnTheFly.Member(ci.Owner.Email) {
							// THIS SHOULD FAIL THE CQ RUN!!!!!!!!!!!!!!!!!!!!!!!!!
							sklog.Errorf("Config was modified in %d but the owner %s does not have permission to run it", ci.Issue, ci.Owner.Email)
							continue
						}
						if err := cfg.Validate(); err != nil {
							// THIS SHOULD FAIL THE CQ RUN!!!!!!!!!!! WIT HTHE ERR
							sklog.Error(err)
							continue
						}
						sklog.Infof("%d in %s has specified a cfg. Use it!", ci.Issue, repoBranch)
						fmt.Println(cfg)
						fmt.Println(cfg.CommitterList)
						fmt.Println(cfg.DryRunAccessList)
						fmt.Println(cfg.Internal)
						fmt.Println(cfg.TasksJSONPath)
						fmt.Println(cfg.TreeStatusURL)

						// Only consider the cfg file if the owner is a committer. Else do not consider it.
						fmt.Println(ci.Owner.Email)
					}
				}
			} else {
				sklog.Infof("NOT SEEN %s BEFORE!!!", repoBranch)

				// NEED TO DO THE WHOLE CONFIG CHECKING THING HERE AS WELL

				cfg, err = config.ReadSkCQCfg(ctx, gitilesRepo, ci.Project, ci.Branch)
				if err != nil {
					// Either the config is missing or there was an error in reading it. Log the error
					// and move on.
					sklog.Errorf("Error reading SkCQ cfg for %s", repoBranch)
					// Cache this repo+branch combination as being unsupported.
					ProjectsBranchesConfigCache[repoBranch] = nil
					continue
				}
				// Cache this repo+branch combination.
				ProjectsBranchesConfigCache[repoBranch] = cfg
			}

			// At this point we have the config to use for the CL.
			fmt.Println("AT THIS POINT WE HAVE TH ECONFIG TO USE FOR THE CL")
			fmt.Printf("\n%+v\n", cfg)

			// THESE VERIFIES SHOULD BE CACHED WITH THIS CL+PATCHSET COMBO. HOW DO YOU CLEANUP OLD CL+PATCHSETS? BY PUTING IN A MAP OR SOMETHING
			clVerifiers, err := verifiers.GetVerifiers(ctx, httpClient, cfg, cr, ci)
			if err != nil {
				sklog.Errorf("[%d] Error when getting verifiers: %s", ci.Issue, err)
				cr.RemoveFromCQ(ctx, ci, "Error when getting verifiers. Removing from CQ. Please ask Infra Gardener to investigate.")
				continue
			}
			fmt.Println("THESE ARE THE VERIFIERS")
			fmt.Println(clVerifiers)

			successMsgsFromVerifiers, waitMsgsFromVerifiers, rejectMsgsFromVerifiers := verifiers.RunVerifiers(ci, clVerifiers)
			if len(rejectMsgsFromVerifiers) > 0 {
				sklog.Infof("%d from %s has failed verifiers: %+v", ci.Issue, repoBranch, rejectMsgsFromVerifiers)
				cr.RemoveFromCQ(ctx, ci, strings.Join(rejectMsgsFromVerifiers, "\n"))
			} else if len(waitMsgsFromVerifiers) > 0 {
				sklog.Infof("%d from %s is waiting for verifiers: %+v", ci.Issue, repoBranch, waitMsgsFromVerifiers)
			} else {
				sklog.Infof("%d from %s successfully ran verifiers: %+v", ci.Issue, repoBranch, successMsgsFromVerifiers)
				// All verifiers were successful. Either submit or say it is done.
				if cr.IsDryRun(ctx, ci) {
					// Say everything was succesful and we are done.
					cr.RemoveFromCQ(ctx, ci, "Dry run: This CL passed the SkCQ dry run.")
					// REMOVE FROM CACHE SOMEHOW.
				} else {
					// Do something for regular CQ.
					// Also check for all the things that need to be submitted together - that new API
					// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#submitted-together
					// Make sure all the changes there pass all the verifiers we would have run for them!!!!
					togetherChanges, err := cr.GetSubmittedTogether(ctx, ci)
					if err != nil {
						sklog.Errorf("[%d] Error when getting submitted together changes: %s", ci.Issue, err)
						cr.RemoveFromCQ(ctx, ci, "Error when submitting. Removing from CQ. Please ask Infra Gardener to investigate.")
					}
					fmt.Println("THESE ARE THE TOGETHER CHANGES!")
					fmt.Println(togetherChanges)
					// THIS SHOULD PROBABLY BE A DIFFERENT VIERIFIER FOR SUBMITTEDTOGETHERVERIFIER!
					for _, ts := range togetherChanges {
						// Get the verifiers that apply to this together change.
						tsVerifiers, err := verifiers.GetVerifiers(ctx, httpClient, cfg, cr, ts)
						if err != nil {
							sklog.Errorf("[%s/%d] Error when getting verifiers for submitted together change %d: %s", repoBranch, ci.Issue, ts.Issue, err)
							cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("Error when getting verifiers for %d. Removing from CQ. Please ask Infra Gardener to investigate.", ts.Issue))
							continue
						}
						sm, wm, rm := verifiers.RunVerifiers(ts, tsVerifiers)
						if len(rm) > 0 {
							sklog.Infof("[%s/%d] Submitted together change %s has failed verifiers: %+v", repoBranch, ci.Issue, ts.Issue, rm)
							cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("The dependent CL %d has failed:\n%s", ts.Issue, strings.Join(rejectMsgsFromVerifiers, "\n")))
						} else if len(wm) > 0 {
							sklog.Infof("[%s/%d] Submitted together change %s is waiting for verifiers: %+v", repoBranch, ci.Issue, ts.Issue, wm)
						} else {
							sklog.Infof("[%s/%d] Submitted together change %s successfully ran verifiers: %+v", repoBranch, ci.Issue, ts.Issue, sm)
						}
					}

					if err := cr.Submit(ctx, ci); err != nil {
						sklog.Errorf("[%d] Error when submitted: %s", ci.Issue, err)
						cr.RemoveFromCQ(ctx, ci, "Error when submitting. Removing from CQ. Please ask Infra Gardener to investigate.")
					}
				}
			}
		}

	}, nil)

	return nil
}
