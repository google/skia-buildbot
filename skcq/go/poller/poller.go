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
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/db"
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
	// GOB THIS BEFORE LAUNCH.
	ChangeAndPatchsetToCQRecord = map[string]*CQRecord{}
)

type CQRecord struct {
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
func Start(ctx context.Context, pollInterval time.Duration, cr codereview.CodeReview, httpClient *http.Client, dbClient *db.FirestoreDB, canModifyCfgsOnTheFly *allowed.AllowedFromChromeInfraAuth) error {

	// Do we have an already populated ChangeAndPatchsetToCQRecord cache somehwerE?
	currentChanges, err := dbClient.GetCurrentChanges(ctx)
	if err != nil {
		return skerr.Fmt("Could not get current changes: %s", err)
	}
	if currentChanges == nil {
		sklog.Info("This is the first time this instance is coming up. Creating a new cache of changesToCQRecords")
		ChangeAndPatchsetToCQRecord = map[string]*CQRecord{}
	} else {
		// Do some gob decoding or something like that.
	}
	fmt.Println("WE GOT THEDSE CHANGES: ")
	fmt.Printf("%+v", currentChanges)
	sklog.Fatal("GIVING UP HERE TO SEE WHAT WE GET ABOVE!")

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

			// Update the cache if it is not already in there.
			cqRecord, ok := ChangeAndPatchsetToCQRecord[changePatchset]
			if ok {
				cqRecord.changeVerifiers = clVerifiers
			} else {
				cqRecord = &CQRecord{
					ci:              ci,
					startTime:       time.Now().Unix(),
					changeVerifiers: clVerifiers,
				}
			}
			ChangeAndPatchsetToCQRecord[changePatchset] = cqRecord

			// Now run the verifiers.
			successMsgsFromVerifiers, waitMsgsFromVerifiers, rejectMsgsFromVerifiers := verifiers.RunVerifiers(ctx, ci, clVerifiers, cqRecord.startTime)
			if len(rejectMsgsFromVerifiers) > 0 {
				sklog.Infof("%d from %s has failed verifiers: %+v", ci.Issue, repoBranch, rejectMsgsFromVerifiers)
				cr.RemoveFromCQ(ctx, ci, strings.Join(rejectMsgsFromVerifiers, "\n"))
				removeFromChangesCache(ctx, changePatchset, false)
			} else if len(waitMsgsFromVerifiers) > 0 {
				sklog.Infof("%d from %s is waiting for verifiers: %s", ci.Issue, repoBranch, strings.Join(waitMsgsFromVerifiers, ", "))
			} else {
				sklog.Infof("%d from %s successfully ran verifiers: %s", ci.Issue, repoBranch, strings.Join(successMsgsFromVerifiers, ", "))
				// All verifiers were successful. Either submit or say it is done.
				if cr.IsDryRun(ctx, ci) {
					// Say everything was succesful and we are done.
					cr.RemoveFromCQ(ctx, ci, "Dry run: This CL passed the SkCQ dry run.")
					removeFromChangesCache(ctx, changePatchset, false)
					// REMOVE FROM CACHE SOMEHOW.
				} else {
					if err := cr.Submit(ctx, ci); err != nil {
						sklog.Errorf("[%d] Error when submitted: %s", ci.Issue, err)
						cr.RemoveFromCQ(ctx, ci, "Error when submitting. Removing from CQ. Please ask Infra Gardener to investigate.")
						removeFromChangesCache(ctx, changePatchset, false)
					}
				}
			}
		}

		// Find missing CLs and run cleanup on their verifiers!
		for changePatchset, _ := range ChangeAndPatchsetToCQRecord {
			// Was this change processed in the current round?
			if _, ok := clsInThisRound[changePatchset]; !ok {
				// The change is no longer in the CQ. Might have been manually removed.
				removeFromChangesCache(ctx, changePatchset, true)
			}
		}

	}, nil)

	return nil
}

func addToChangesCache(changePatchset string, ci *gerrit.ChangeInfo, changeVerifiers []verifiers.Verifier) {
	// Update the cache if it is not already in there.
	cqRecord, ok := ChangeAndPatchsetToCQRecord[changePatchset]
	if ok {
		cqRecord.changeVerifiers = changeVerifiers
	} else {
		cqRecord = &CQRecord{
			ci:              ci,
			startTime:       time.Now().Unix(),
			changeVerifiers: changeVerifiers,
		}
	}
	ChangeAndPatchsetToCQRecord[changePatchset] = cqRecord
}

func removeFromChangesCache(ctx context.Context, changePatchset string, runCleanup bool) {
	fmt.Printf("\nREMOVING %s from ChangesCache and running cleanup: %t", changePatchset, runCleanup)
	if cqRecord, ok := ChangeAndPatchsetToCQRecord[changePatchset]; ok {
		if runCleanup {
			for _, v := range cqRecord.changeVerifiers {
				v.Cleanup(ctx, cqRecord.ci)
			}
		}
		delete(ChangeAndPatchsetToCQRecord, changePatchset)
	}
}
