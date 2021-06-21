package poller

// Initializes and polls the various issue frameworks.

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/util"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/throttler"
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
func Start(ctx context.Context, pollInterval time.Duration, cr codereview.CodeReview, httpClient *http.Client, dbClient *db.FirestoreDB, canModifyCfgsOnTheFly *allowed.AllowedFromChromeInfraAuth, publicFEInstanceURL, corpFEInstanceURL string, reposAllowList, reposBlockList []string) error {

	// This is setting the CurrentChangesCache from current_changes_cache.
	// This needs to be in a better place.
	var err error
	CurrentChangesCache, err = dbClient.GetCurrentChanges(ctx)
	if err != nil {
		return skerr.Fmt("Could not get current changes: %s", err)
	}
	sklog.Infof("CurrentChangesCache: %+v", CurrentChangesCache)

	// Keep track of commit timestamps for the throttler.
	// repoBranchToCommitTimes := map[string][]time.Time{}

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
			if len(reposAllowList) > 0 && !util.In(ci.Project, reposAllowList) {
				sklog.Infof("The repo %s is not in the repos allowlist: %s. Ignoring %d.", ci.Project, reposAllowList, ci.Issue)
				continue
			}
			if len(reposBlockList) > 0 && util.In(ci.Project, reposBlockList) {
				sklog.Infof("The repo %s is in the repos blocklist: %s. Ignoring %d.", ci.Project, reposBlockList, ci.Issue)
				continue
			}
			// if ci.Project != "skiabot-playground" && ci.Project != "skiabot-test" {
			// 	continue
			// }

			repoBranch := fmt.Sprintf("%s/%s", ci.Project, ci.Branch)
			// Use the equivalent patchset for the changes cache because
			// if NO_CODE change patches come in then we want to treat it the
			// same as the earliest equivalent patch.
			changeEquivalentPatchset := fmt.Sprintf("%d/%d", ci.Issue, cr.GetEarliestEquivalentPatchSetID(ci))
			clsInThisRound[changeEquivalentPatchset] = true

			sklog.Infof("[%d] Started processing in repo+branch %s", ci.Issue, repoBranch)

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
					cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("%s. Removing from CQ.\nPlease add a infra/skcq.cfg file if this repo+branch requires CQ.", err.Error()))
					continue
				} else if _, ok := err.(*config.CannotModifyCfgsOnTheFlyError); ok {
					cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("CL owner %s does not have permission to modify %s", ci.Owner.Email, config.SkCQCfgPath))
					continue
				} else {
					cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("Error reading %s for %d. Removing from CQ. Please ask Infra Gardener to investigate.", config.SkCQCfgPath, ci.Issue))
					continue
				}
			}

			// At this point we have the config to use for the CL, gather all verifiers for this change.
			clVerifiers, togetherChanges, err := verifiers.GetVerifiers(ctx, httpClient, skCQCfg, cr, ci, false /* isSubmittedTogetherChange */, configReader)
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
			cqStartTime, newCQRun := AddToChangesCache(ctx, changeEquivalentPatchset, ci.Subject, ci.Owner.Email, ci.Project, ci.Branch, dbClient, cr.IsDryRun(ctx, ci), skCQCfg.Internal, ci.Issue, cr.GetLatestPatchSetID(ci))
			cqEndTime := int64(0)
			cqSubmittedTime := int64(0)

			if newCQRun {
				// If this is a new CQ run then before running the verifiers update the
				// CL with an auto-generated comment saying we are processing this patch.
				feURL := publicFEInstanceURL
				if skCQCfg.Internal {
					feURL = corpFEInstanceURL
				}
				notify := "NONE"
				comment := "SkCQ is trying the patch."
				if cr.IsDryRun(ctx, ci) {
					comment = fmt.Sprintf("Dry run: %s", comment)
				} else if len(togetherChanges) > 0 {
					togetherChangesLinks := []string{}
					for _, t := range togetherChanges {
						togetherChangesLinks = append(togetherChangesLinks, fmt.Sprintf("https://skia-review.googlesource.com/c/%s", t))
					}
					comment = fmt.Sprintf("%s\nThis change will be submitted with the following changes: %s", comment, strings.Join(togetherChangesLinks, ", "))
					// Notify owner and reviewers to let them know that other changes might be submitted as well.
					notify = "OWNER_REVIEWERS"
				}
				comment = fmt.Sprintf("%s\n\nFollow status at: %s/%d/%d", comment, feURL, ci.Issue, cr.GetLatestPatchSetID(ci))
				cr.AddComment(ctx, ci, comment, notify)
			}

			// Now run the verifiers.
			verifierStatuses := verifiers.RunVerifiers(ctx, ci, clVerifiers, cqStartTime)
			rejectMsgsFromVerifiers, waitMsgsFromVerifiers, successMsgsFromVerifiers := verifiers.GetStatusStringsFromVerifierStatuses(verifierStatuses)
			var attemptOverallState types.VerifierState
			if len(rejectMsgsFromVerifiers) > 0 {
				sklog.Infof("[%d] from %s has failed verifiers: %+v", ci.Issue, repoBranch, rejectMsgsFromVerifiers)
				cr.RemoveFromCQ(ctx, ci, strings.Join(rejectMsgsFromVerifiers, "\n"))
				RemoveFromChangesCache(ctx, changeEquivalentPatchset, false, dbClient)
				attemptOverallState = types.VerifierFailureState
				cqEndTime = time.Now().Unix()
			} else if len(waitMsgsFromVerifiers) > 0 {
				sklog.Infof("[%d] from %s is waiting for verifiers: %s", ci.Issue, repoBranch, strings.Join(waitMsgsFromVerifiers, ", "))
				attemptOverallState = types.VerifierWaitingState
			} else {
				sklog.Infof("[%d] from %s successfully ran verifiers: %s", ci.Issue, repoBranch, strings.Join(successMsgsFromVerifiers, ", "))
				if cr.IsDryRun(ctx, ci) {
					removeFromCQMsg := "Dry run: This CL passed the SkCQ dry run."
					if ci.WorkInProgress {
						for _, r := range ci.Reviewers.Reviewer {
							// Owner shows up as a reviewer. No idea why Gerrit does this.
							if r.AccountID != ci.Owner.AccountID {
								// Publish and break out.
								if err := cr.SetReadyForReview(ctx, ci); err != nil {
									sklog.Errorf("Could not set for review %d: %s", ci.Issue, err)
								}
								removeFromCQMsg += "\nAutomatically published the CL because it was WIP with reviewers specified."
								break
							}
						}
						// ci.Reviewers
						// cr.PublishIfWIPAndReviewers()
						// Add to the below msg. "Automatically published CL because it was WIP with reviewers attached."
					}
					// Say everything was succesful and we are done.
					cr.RemoveFromCQ(ctx, ci, removeFromCQMsg)
				} else {
					// As a final step before submitting make sure that "Commit: false" is not in the footers.
					if err := cr.Submit(ctx, ci); err != nil {
						sklog.Errorf("[%d] Error when submitted: %s", ci.Issue, err)
						cr.RemoveFromCQ(ctx, ci, "Error when submitting. Removing from CQ. Please ask Infra Gardener to investigate.")
					} else {
						cqSubmittedTime = time.Now().Unix()
						throttler.UpdateThrottler(repoBranch, time.Now(), skCQCfg.ThrottlerCfg)
					}
				}
				RemoveFromChangesCache(ctx, changeEquivalentPatchset, false, dbClient)
				cqEndTime = time.Now().Unix()
				attemptOverallState = types.VerifierSuccessState
			}

			if attemptOverallState == types.VerifierFailureState {
				// Do a pass through of all verifiers and update any states that are in
				// VerifierWatitingState to VerifierAbortedState because we are no longer
				// waiting for this verifier.
				for _, v := range verifierStatuses {
					if v.State == types.VerifierWaitingState {
						v.State = types.VerifierAbortedState
						v.Stop = time.Now().Unix()
					}
				}
			}

			// We are done processing this CL for this iteration of the poller's
			// loop. Persist the CL state for UI display.
			attempt := &types.ChangeAttempt{
				ChangeID:           ci.Issue,
				PatchsetID:         cr.GetLatestPatchSetID(ci),
				DryRun:             cr.IsDryRun(ctx, ci),
				Repo:               ci.Project,
				Branch:             ci.Branch,
				PatchStart:         cqStartTime,
				PatchStop:          cqEndTime,
				PatchCommitted:     cqSubmittedTime,
				SubmittableChanges: togetherChanges,
				VerifiersStatuses:  verifierStatuses,
				OverallState:       attemptOverallState,
			}
			if err := dbClient.PutChangeAttempt(ctx, attempt, skCQCfg.Internal); err != nil {
				sklog.Errorf("Could not persist change %d: %s", ci.Issue, err)
			}
		}

		// Find missing CLs and run cleanup on their verifiers!
		for changeEquivalentPatchset, cqRecord := range CurrentChangesCache {
			// Was this change processed in the current round?
			if _, ok := clsInThisRound[changeEquivalentPatchset]; !ok {
				sklog.Infof("%s is no longer processed by SkCQ. It was processed in the last cycle and was still running. Going to cleanup it's verifiers.", changeEquivalentPatchset)

				// Remove the change from the changes cache.
				RemoveFromChangesCache(ctx, changeEquivalentPatchset, true, dbClient)

				// Update the attempt as being abandoned so that the UI accurately reflects what happened to
				// the change.
				if err := dbClient.UpdateChangeAttemptAsAbandoned(ctx, cqRecord.ChangeID, cqRecord.PatchsetID, cqRecord.Internal, cqRecord.StartTime); err != nil {
					sklog.Errorf("Could not mark change %s as abandoned", changeEquivalentPatchset)
					continue
				}

				// Run cleanup on all verifiers for this abaonded change iff a newer code change patchset has been
				// uploaded here.
				// cqRecord.
				// Get the latest change obj using the change ID.
				ci, err := cr.GetIssueProperties(ctx, cqRecord.ChangeID)
				if err != nil {
					sklog.Errorf("Could not get change for %d: %s", ci.Issue, err)
					continue
				}

				// Get and run the cleanup of verifiers
				configReader, err := config.NewGitilesConfigReader(ctx, httpClient, ci, cr, canModifyCfgsOnTheFly)
				if err != nil {
					sklog.Errorf("Could not get config reader for %d: %s", ci.Issue, err)
					continue
				}
				skCQCfg, err := configReader.GetSkCQCfg(ctx)
				if err != nil {
					sklog.Errorf("Could not get skcqcfg for %d: %s", ci.Issue, err)
					continue
				}

				verifiers, _, err := verifiers.GetVerifiers(ctx, httpClient, skCQCfg, cr, ci, false /* isSubmittedTogetherChange */, configReader)
				if err != nil {
					sklog.Errorf("Could not get verifiers for %d: %s", ci.Issue, err)
					continue
				}
				tokens := strings.Split(changeEquivalentPatchset, "/")
				pID, err := strconv.ParseInt(tokens[1], 10, 64)
				if err != nil {
					sklog.Errorf("Could not parse patcshetID from %s: %s", tokens[1], err)
					continue
				}
				for _, v := range verifiers {
					v.Cleanup(ctx, ci, pID)
				}
			}
		}

	}, nil)

	return nil
}
