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
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/verifiers"
)

const (
	LivenessMetric = "skcq_be"
	SkiaRepoTmpl   = "https://skia.googlesource.com/%s.git"
)

// Start polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func Start(ctx context.Context, pollInterval time.Duration, cr codereview.CodeReview, httpClient *http.Client, dbClient *db.FirestoreDB, canModifyCfgsOnTheFly *allowed.AllowedFromChromeInfraAuth) error {

	// Do we have an already populated ChangeAndPatchsetToCQRecord cache somehwerE?
	// gob.Register(map[string]CQRecord{})
	var err error
	// RETURN EMPTY INSTEAD OF NIL!!!!!
	CurrentChangesCache, err = dbClient.GetCurrentChanges(ctx)
	if err != nil {
		return skerr.Fmt("Could not get current changes: %s", err)
	}
	sklog.Infof("CurrentChangesCache: %+v", CurrentChangesCache)

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

		// Storing CLs that are being processed in this round for quicker lookup.
		clsInThisRound := map[string]bool{}

		// Process the CLs now
		for _, ci := range cls {

			// TO MAKE IT EASIER FOR MY TESTING FOR NOW
			if ci.Project != "skiabot-playground" && ci.Project != "skiabot-test" {
				continue
			}

			repoBranch := fmt.Sprintf("%s/%s", ci.Project, ci.Branch)
			changePatchset := fmt.Sprintf("%d/%d", ci.Issue, cr.GetEarliestEquivalentPatchSetID(ci))
			clsInThisRound[changePatchset] = true

			sklog.Infof("[%d] Started processing in repo+branch %s", ci.Issue, repoBranch)

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
			if err != nil {
				if _, ok := err.(*config.ConfigNotFoundError); ok {
					cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("%s. Removing from CQ. Please add a config if this repo+branch requires CQ.", err.Error()))
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
			clVerifiers, togetherChanges, err := verifiers.GetVerifiers(ctx, httpClient, skCQCfg, cr, ci, false /* isSubmittedTogetherChange */, gitilesRepo, configReader)
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

			// Update the cache if it is not already in there.
			cqStartTime := AddToChangesCache(ctx, changePatchset, dbClient)
			cqEndTime := int64(0)
			cqSubmittedTime := int64(0)

			// Now run the verifiers.
			verifierStatuses := verifiers.RunVerifiers(ctx, ci, clVerifiers, cqStartTime)
			rejectMsgsFromVerifiers, waitMsgsFromVerifiers, successMsgsFromVerifiers := verifiers.GetStatusStringsFromVerifierStatuses(verifierStatuses)
			if len(rejectMsgsFromVerifiers) > 0 {
				sklog.Infof("[%d] from %s has failed verifiers: %+v", ci.Issue, repoBranch, rejectMsgsFromVerifiers)
				cr.RemoveFromCQ(ctx, ci, strings.Join(rejectMsgsFromVerifiers, "\n"))
				RemoveFromChangesCache(ctx, changePatchset, false, dbClient)
			} else if len(waitMsgsFromVerifiers) > 0 {
				sklog.Infof("[%d] from %s is waiting for verifiers: %s", ci.Issue, repoBranch, strings.Join(waitMsgsFromVerifiers, ", "))
			} else {
				sklog.Infof("[%d] from %s successfully ran verifiers: %s", ci.Issue, repoBranch, strings.Join(successMsgsFromVerifiers, ", "))
				if cr.IsDryRun(ctx, ci) {
					// Say everything was succesful and we are done.
					cr.RemoveFromCQ(ctx, ci, "Dry run: This CL passed the SkCQ dry run.")
				} else {
					if err := cr.Submit(ctx, ci); err != nil {
						sklog.Errorf("[%d] Error when submitted: %s", ci.Issue, err)
						cr.RemoveFromCQ(ctx, ci, "Error when submitting. Removing from CQ. Please ask Infra Gardener to investigate.")
					} else {
						cqSubmittedTime = time.Now().Unix()
					}
				}
				RemoveFromChangesCache(ctx, changePatchset, false, dbClient)
				cqEndTime = time.Now().Unix()
			}

			// We are done processing this CL for this iteration of the poller's
			// loop. Persist the CL state for UI display.
			// All of this will exist.
			// fmt.Println(cqStartTime) // This is wrong. Use the one from the record??
			// fmt.Println(cqEndTime)
			// fmt.Println(cqSubmittedTime)
			// fmt.Println(togetherChanges)
			// fmt.Println(verifierStatuses)
			attempt := &types.ChangeAttempt{
				ID:                 fmt.Sprintf("%d", ci.Issue),
				DryRun:             cr.IsDryRun(ctx, ci),
				RepoBranch:         repoBranch,
				PatchStart:         cqStartTime,
				PatchStop:          cqEndTime,
				PatchCommitted:     cqSubmittedTime,
				SubmittableChanges: togetherChanges,
				VerifiersStatuses:  verifierStatuses,
			}
			if err := dbClient.PutChangeAttempt(ctx, attempt, skCQCfg.Internal); err != nil {
				sklog.Errorf("Could not persist change %d: %s", ci.Issue, err)
			}
		}

		// Find missing CLs and run cleanup on their verifiers!
		for changePatchset, _ := range CurrentChangesCache {
			// Was this change processed in the current round?
			if _, ok := clsInThisRound[changePatchset]; !ok {
				// The change is no longer in the CQ. Might have been manually removed.
				RemoveFromChangesCache(ctx, changePatchset, true, dbClient)
			}
		}

	}, nil)

	return nil
}
