package roller

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/commit_msg"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/modes"
	arb_notifier "go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller_cleanup"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/time_window"
	"go.skia.org/infra/autoroll/go/unthrottle"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/comment"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// AutorollURLPublic is the public autoroll frontend URL.
	AutorollURLPublic = "https://autoroll.skia.org"
	// AutorollURLPrivate is the private autoroll frontend URL.
	AutorollURLPrivate = "https://skia-autoroll.corp.goog"

	// maxNotRolledRevs is the maximum number of not-yet-rolled revisions to
	// store in the DB.
	maxNotRolledRevs = 50

	// We'll send a notification if this many rolls fail in a row.
	notifyIfLastNFailed = 3

	// Unless otherwise configured, we'll wait 10 minutes after a successful
	// roll before uploading a new roll. This prevents safety-throttling for
	// rollers which have a very fast commit queue.
	defaultRollCooldown = 10 * time.Minute
)

// AutoRoller is a struct which automates the merging new revisions of one
// project into another.
type AutoRoller struct {
	cfg                   *config.Config
	cleanup               roller_cleanup.DB
	client                *http.Client
	codereview            codereview.CodeReview
	commitMsgBuilder      *commit_msg.Builder
	currentRoll           codereview.RollImpl
	dryRunSuccessThrottle *state_machine.Throttler
	emails                []string
	emailsMtx             sync.RWMutex
	failureThrottle       *state_machine.Throttler
	lastRollRev           *revision.Revision
	liveness              metrics2.Liveness
	manualRollDB          manual.DB
	modeHistory           modes.ModeHistory
	nextRollRev           *revision.Revision
	notifier              *arb_notifier.AutoRollNotifier
	notRolledRevs         []*revision.Revision
	recent                *recent_rolls.RecentRolls
	rm                    repo_manager.RepoManager
	roller                string
	rollUploadAttempts    metrics2.Counter
	rollUploadFailures    metrics2.Counter
	runningMtx            sync.Mutex
	safetyThrottle        *state_machine.Throttler
	serverURL             string
	reportedRevs          map[string]time.Time
	reviewers             []string
	reviewersBackup       []string
	sm                    *state_machine.AutoRollStateMachine
	status                *status.Cache
	statusMtx             sync.RWMutex
	strategy              strategy.NextRollStrategy
	strategyHistory       *strategy.DatastoreStrategyHistory
	strategyMtx           sync.RWMutex // Protects strategy
	successThrottle       *state_machine.Throttler
	throttle              unthrottle.Throttle
	timeWindow            *time_window.TimeWindow
	tipRev                *revision.Revision
	workdir               string
}

// NewAutoRoller returns an AutoRoller instance.
func NewAutoRoller(ctx context.Context, c *config.Config, emailer email.Client, chatBotConfigReader chatbot.ConfigReader, g gerrit.GerritInterface, githubClient *github.GitHub, workdir, serverURL string, gcsClient gcs.GCSClient, client *http.Client, rollerName string, local bool, statusDB status.DB, manualRollDB manual.DB, cleanupDB roller_cleanup.DB) (*AutoRoller, error) {
	// Validation and setup.
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "Failed to validate config")
	}
	var cr codereview.CodeReview
	var err error
	if c.GetGerrit() != nil {
		cr, err = codereview.NewGerrit(c.GetGerrit(), g, client)
	} else if c.GetGithub() != nil {
		cr, err = codereview.NewGitHub(c.GetGithub(), githubClient)
	} else {
		return nil, skerr.Fmt("Either GitHub or Gerrit is required.")
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to initialize code review")
	}

	// Create the AutoRoller struct.
	arb := &AutoRoller{
		cfg:                c,
		cleanup:            cleanupDB,
		client:             client,
		codereview:         cr,
		liveness:           metrics2.NewLiveness("last_autoroll_landed", map[string]string{"roller": c.RollerName}),
		manualRollDB:       manualRollDB,
		roller:             rollerName,
		rollUploadAttempts: metrics2.GetCounter("autoroll_cl_upload_attempts", map[string]string{"roller": c.RollerName}),
		rollUploadFailures: metrics2.GetCounter("autoroll_cl_upload_failures", map[string]string{"roller": c.RollerName}),
		serverURL:          serverURL,
		reviewers:          c.Reviewer,
		reviewersBackup:    c.ReviewerBackup,
		throttle:           unthrottle.NewDatastore(ctx),
		workdir:            workdir,
	}

	// Helper function in case we fail to create or update the RepoManager.
	deleteLocalDataOnFailure := func(operation string, err error) error {
		if err != nil {
			sklog.Errorf("Failed %s; deleting local data and exiting", operation)
			if cleanupErr := arb.DeleteLocalData(ctx); cleanupErr != nil {
				sklog.Errorf("Failed to delete local data: %s", cleanupErr)
			}
			return err
		}
		return nil
	}

	// Create the RepoManager.
	rm, err := repo_manager.New(ctx, c.GetRepoManagerConfig(), workdir, rollerName, serverURL, c.ServiceAccount, client, cr, c.IsInternal, local)
	if err := deleteLocalDataOnFailure("creating RepoManager", err); err != nil {
		return nil, skerr.Wrap(err)
	}
	arb.rm = rm

	sklog.Info("Creating strategy history.")
	sh, err := strategy.NewDatastoreStrategyHistory(ctx, rollerName, c.ValidStrategies())
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create strategy history")
	}
	arb.strategyHistory = sh

	currentStrategy := sh.CurrentStrategy()
	if currentStrategy == nil {
		// If there's no history, set the initial strategy.
		sklog.Infof("Setting initial strategy for %s to %q", rollerName, c.DefaultStrategy())
		if err := sh.Add(ctx, c.DefaultStrategy(), "AutoRoll Bot", "Setting initial strategy."); err != nil {
			return nil, skerr.Wrapf(err, "Failed to set initial strategy")
		}
		currentStrategy = sh.CurrentStrategy()
	}

	sklog.Info("Setting strategy.")
	strat, err := strategy.GetNextRollStrategy(currentStrategy.Strategy)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get next roll strategy")
	}
	arb.strategy = strat

	sklog.Info("Running repo_manager.Update()")
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	if err := deleteLocalDataOnFailure("initial RepoManager update", err); err != nil {
		return nil, skerr.Wrap(err)
	}
	arb.lastRollRev = lastRollRev
	arb.tipRev = tipRev
	arb.notRolledRevs = notRolledRevs

	nextRollRev := strat.GetNextRollRev(notRolledRevs)
	if nextRollRev == nil {
		nextRollRev = lastRollRev
	}
	arb.nextRollRev = nextRollRev

	// Adding notRolledRevs to reportedRevs here prevents double reporting
	// of latencies across the roller going offline and starting up. This approach
	// increases the accuracy of the revision detection latency metric at the cost
	// of potentially missing revisions.
	reportedRevs := make(map[string]time.Time)
	for _, rev := range notRolledRevs {
		reportedRevs[rev.Id] = rev.Timestamp
	}
	arb.reportedRevs = reportedRevs

	sklog.Info("Creating roll history")
	recent, err := recent_rolls.NewRecentRolls(ctx, recent_rolls.NewDatastoreRollsDB(ctx), rollerName)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create recent rolls DB")
	}
	arb.recent = recent

	sklog.Info("Creating mode history")
	mh, err := modes.NewDatastoreModeHistory(ctx, rollerName)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create mode history")
	}
	if mh.CurrentMode() == nil {
		sklog.Info("Setting initial mode.")
		if err := mh.Add(ctx, modes.ModeRunning, "AutoRoll Bot", "Setting initial mode."); err != nil {
			return nil, skerr.Wrapf(err, "Failed to set initial mode")
		}
	}
	arb.modeHistory = mh

	// Throttling counters.
	sklog.Info("Creating throttlers")
	safetyThrottleCfg := config.DefaultSafetyThrottleConfig
	if c.SafetyThrottle != nil {
		safetyThrottleCfg = c.SafetyThrottle
	}
	safetyThrottleDuration, err := human.ParseDuration(safetyThrottleCfg.TimeWindow)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse safety throttle time window")
	}
	safetyThrottle, err := state_machine.NewThrottler(ctx, gcsClient, rollerName+"/attempt_counter", safetyThrottleDuration, int64(safetyThrottleCfg.AttemptCount))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create safety throttler")
	}
	arb.safetyThrottle = safetyThrottle

	failureThrottle, err := state_machine.NewThrottler(ctx, gcsClient, rollerName+"/fail_counter", time.Hour, 1)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create failure throttler")
	}
	arb.failureThrottle = failureThrottle

	rollCooldown := defaultRollCooldown
	if c.RollCooldown != "" {
		rollCooldown, err = human.ParseDuration(c.RollCooldown)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to parse roll cooldown")
		}
	}
	successThrottle, err := state_machine.NewThrottler(ctx, gcsClient, rollerName+"/success_counter", rollCooldown, 1)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create success throttler")
	}
	arb.successThrottle = successThrottle

	var dryRunCooldown time.Duration
	if c.DryRunCooldown != "" {
		dryRunCooldown, err = human.ParseDuration(c.DryRunCooldown)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to parse dry run cooldown")
		}
	}
	dryRunSuccessThrottle, err := state_machine.NewThrottler(ctx, gcsClient, rollerName+"/dry_run_success_counter", dryRunCooldown, 1)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create dry run success throttler")
	}
	arb.dryRunSuccessThrottle = dryRunSuccessThrottle

	sklog.Info("Getting reviewers")
	emails := GetReviewers(client, c.RollerName, c.Reviewer, c.ReviewerBackup)
	arb.emails = emails
	sklog.Info("Creating notifier")
	configCopies := replaceReviewersPlaceholder(c.Notifiers, emails)
	n, err := arb_notifier.New(ctx, c.ChildDisplayName, c.ParentDisplayName, serverURL, client, emailer, chatBotConfigReader, configCopies)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create notifier")
	}
	arb.notifier = n

	sklog.Info("Creating status cache.")
	statusCache, err := status.NewCache(ctx, statusDB, rollerName)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create status cache")
	}
	arb.status = statusCache

	sklog.Info("Creating TimeWindow.")
	var tw *time_window.TimeWindow
	if c.TimeWindow != "" {
		tw, err = time_window.Parse(c.TimeWindow)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create TimeWindow")
		}
	}
	arb.timeWindow = tw

	commitMsgBuilder, err := commit_msg.NewBuilder(c.CommitMsg, c.ChildDisplayName, c.ParentDisplayName, serverURL, c.ChildBugLink, c.ParentBugLink, c.TransitiveDeps)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	arb.commitMsgBuilder = commitMsgBuilder

	sklog.Info("Creating state machine")
	sm, err := state_machine.New(ctx, arb, n, gcsClient, rollerName)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create state machine")
	}
	arb.sm = sm

	current := recent.CurrentRoll()
	if current != nil {
		// Update our view of the active roll.

		// We've encountered problems in the past where a dependency was
		// specified incorrectly. The roller uploads a CL to change the wrong
		// dependency, a human notices and fixes the roller config, but now
		// there's an active roll whose RollingFrom and RollingTo revisions are
		// not valid for the new, correct dependency. In this case, we need to
		// temporarily ignore the error we get when we attempt to retrieve these
		// revisions, retrieve the roll (relying on the fact that retrieveRoll
		// doesn't actually make use of these revisions), then abandon it. This
		// allows the roller to self-correct instead of crash-looping, requiring
		// a human to make manual changes in the DB.
		rollingFrom, rollingFromErr := arb.getRevision(ctx, current.RollingFrom)
		rollingTo, rollingToErr := arb.getRevision(ctx, current.RollingTo)
		roll, err := arb.retrieveRoll(ctx, current, rollingFrom, rollingTo)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to retrieve current roll")
		}
		if (rollingFromErr != nil || rollingToErr != nil) && !roll.IsClosed() {
			retrieveErr := rollingFromErr
			failedRev := current.RollingFrom
			if retrieveErr == nil {
				retrieveErr = rollingToErr
				failedRev = current.RollingTo
			}
			msg := fmt.Sprintf("closing the active CL because we failed to retrieve revision %s", failedRev)
			sklog.Errorf("%s: %s", msg, retrieveErr)
			if err := roll.Close(ctx, autoroll.ROLL_RESULT_FAILURE, msg+" - see the roller logs for more information."); err != nil {
				return nil, skerr.Wrapf(err, "failed to retrieve revision and failed to close the active CL")
			}
			// Note: we don't set arb.currentRoll here, so the state machine
			// should transition to S_CURRENT_ROLL_MISSING and then carry on
			// as normal.
		} else {
			if err := roll.InsertIntoDB(ctx); err != nil {
				return nil, err
			}
			arb.currentRoll = roll
		}
	}
	sklog.Info("Done creating autoroller")
	return arb, nil
}

// Retrieve a RollImpl based on the given AutoRollIssue. The passed-in
// AutoRollIssue becomes owned by the RollImpl; it may modify it, insert it
// into the RecentRolls DB, etc. The Issue field is required, and if the roll
// has not yet been inserted into the DB, the RollingFrom, and RollingTo fields
// must be set as well.
func (r *AutoRoller) retrieveRoll(ctx context.Context, roll *autoroll.AutoRollIssue, rollingFrom *revision.Revision, rollingTo *revision.Revision) (codereview.RollImpl, error) {
	return r.codereview.RetrieveRoll(ctx, roll, r.recent, rollingFrom, rollingTo, r.rollFinished)
}

// isSyncError returns true iff the error looks like a sync error.
func isSyncError(err error) bool {
	// TODO(borenet): Remove extra logging.
	sklog.Infof("Encountered error: %q", err.Error())
	if strings.Contains(err.Error(), "Invalid revision range") {
		// Not really an error in the sync itself but indicates that
		// the repo is not up to date, likely due to a server frontend
		// lagging behind.
		sklog.Infof("Is sync error (invalid revision range)")
		return true
	} else if strings.Contains(err.Error(), "The remote end hung up unexpectedly") {
		sklog.Infof("Is sync error (remote hung up)")
		return true
	} else if strings.Contains(err.Error(), "remote error: internal server error") {
		sklog.Infof("Is sync error (internal server error)")
		return true
	} else if strings.Contains(err.Error(), "The requested URL returned error: 502") {
		sklog.Infof("Is sync error (URL returned 502)")
		return true
	} else if strings.Contains(err.Error(), "fatal: bad object") {
		// Not really an error in the sync itself but indicates that
		// the repo is not up to date, likely due to a server frontend
		// lagging behind.
		sklog.Infof("Is sync error (bad object)")
		return true
	} else if strings.Contains(err.Error(), "Got error response status code 404") {
		sklog.Infof("Is sync error (spurious 404 from Gitiles)")
		return true
	}
	sklog.Infof("Not a sync error.")
	return false
}

// Start initiates the AutoRoller's loop.
func (r *AutoRoller) Start(ctx context.Context, tickFrequency time.Duration) {
	sklog.Infof("Starting autoroller.")
	localCheckout := "false"
	if r.cfg.Kubernetes != nil && r.cfg.Kubernetes.Disk != "" {
		localCheckout = "true"
	}
	tags := map[string]string{
		"roller":         r.roller,
		"local_checkout": localCheckout,
	}
	lv := metrics2.NewLiveness("last_successful_autoroll_tick", tags)
	cleanup.Repeat(tickFrequency, func(_ context.Context) {
		// Explicitly ignore the passed-in context; this allows us to
		// continue running even if the context is canceled, which helps
		// to prevent errors due to interrupted syncs, etc.
		ctx := context.Background()
		if err := r.Tick(ctx); err != nil {
			// Hack: we frequently get failures from GoB which trigger error-rate alerts.
			// These alerts are noise and sometimes hide real failures. If the error is
			// due to a sync failure, log it as a warning instead of an error. We'll rely
			// on the liveness alert in the case where we see persistent sync failures.
			if isSyncError(err) {
				sklog.Warningf("Failed to run autoroll: %s", err)
			} else {
				sklog.Errorf("Failed to run autoroll: %s", err)
			}
		} else {
			lv.Reset()
		}
	}, nil)

	// Update the current reviewers in a loop.
	lvReviewers := metrics2.NewLiveness("last_successful_reviewers_retrieval", map[string]string{"roller": r.roller})
	cleanup.Repeat(30*time.Minute, func(ctx context.Context) {
		emails := GetReviewers(r.client, r.cfg.RollerName, r.cfg.Reviewer, r.cfg.ReviewerBackup)
		r.emailsMtx.Lock()
		defer r.emailsMtx.Unlock()
		r.emails = emails

		configCopies := replaceReviewersPlaceholder(r.cfg.Notifiers, emails)
		if err := r.notifier.ReloadConfigs(ctx, configCopies); err != nil {
			sklog.Errorf("Failed to reload configs: %s", err)
			return
		}
		lvReviewers.Reset()
	}, nil)

	// Handle requests for manual rolls.
	if r.cfg.SupportsManualRolls {
		lvManualRolls := metrics2.NewLiveness("last_successful_manual_roll_check", map[string]string{"roller": r.roller})
		cleanup.Repeat(time.Minute, func(_ context.Context) {
			// Explicitly ignore the passed-in context; this allows
			// us to continue handling manual rolls even if the
			// context is canceled, which helps to prevent errors
			// due to interrupted syncs, etc.
			ctx := context.Background()
			if err := r.handleManualRolls(ctx); err != nil {
				sklog.Error(err)
			} else {
				lvManualRolls.Reset()
			}
		}, nil)
	}
}

// Utility for replacing the placeholder $REVIEWERS with real reviewer emails
// in configs. A modified copy of the passed in configs are returned.
func replaceReviewersPlaceholder(configs []*config.NotifierConfig, emails []string) []*notifier.Config {
	configCopies := []*notifier.Config{}
	for _, n := range configs {
		configCopy := arb_notifier.ProtoToConfig(n)
		if configCopy.Email != nil {
			newEmails := []string{}
			for _, e := range configCopy.Email.Emails {
				if e == "$REVIEWERS" {
					newEmails = append(newEmails, emails...)
				} else {
					newEmails = append(newEmails, e)
				}
			}
			configCopy.Email.Emails = newEmails
		}
		configCopies = append(configCopies, configCopy)
	}
	return configCopies
}

// GetActiveRoll implements state_machine.AutoRollerImpl.
func (r *AutoRoller) GetActiveRoll() state_machine.RollCLImpl {
	return r.currentRoll
}

// GetEmails returns the list of email addresses which are copied on rolls.
func (r *AutoRoller) GetEmails() []string {
	r.emailsMtx.RLock()
	defer r.emailsMtx.RUnlock()
	rv := make([]string, len(r.emails))
	copy(rv, r.emails)
	return rv
}

// GetMode implements state_machine.AutoRollerImpl.
func (r *AutoRoller) GetMode() string {
	return r.modeHistory.CurrentMode().Mode
}

// Reset all of the roller's throttle timers.
func (r *AutoRoller) unthrottle(ctx context.Context) error {
	if err := r.failureThrottle.Reset(ctx); err != nil {
		return err
	}
	if err := r.safetyThrottle.Reset(ctx); err != nil {
		return err
	}
	if err := r.successThrottle.Reset(ctx); err != nil {
		return err
	}
	if err := r.dryRunSuccessThrottle.Reset(ctx); err != nil {
		return err
	}
	return nil
}

// UploadNewRoll implements state_machine.AutoRollerImpl.
func (r *AutoRoller) UploadNewRoll(ctx context.Context, from, to *revision.Revision, dryRun bool) (state_machine.RollCLImpl, error) {
	issue, err := r.createNewRoll(ctx, from, to, r.GetEmails(), dryRun, false, "")
	if err != nil {
		return nil, err
	}
	roll, err := r.retrieveRoll(ctx, issue, from, to)
	if err != nil {
		return nil, err
	}
	if err := roll.InsertIntoDB(ctx); err != nil {
		return nil, err
	}
	r.currentRoll = roll
	return roll, nil
}

// createNewRoll is a helper function which uploads a new roll.
func (r *AutoRoller) createNewRoll(ctx context.Context, from, to *revision.Revision, emails []string, dryRun, canary bool, manualRollRequester string) (rv *autoroll.AutoRollIssue, rvErr error) {
	// Track roll CL upload attempts vs failures.
	defer func() {
		r.rollUploadAttempts.Inc(1)
		if rvErr == nil {
			r.rollUploadFailures.Reset()
		} else {
			r.rollUploadFailures.Inc(1)
		}
	}()
	r.statusMtx.RLock()
	var revs []*revision.Revision
	found := false
	for _, rev := range r.notRolledRevs {
		if rev.Id == to.Id {
			found = true
		}
		if found {
			revs = append(revs, rev)
		}
	}
	r.statusMtx.RUnlock()

	commitMsg, err := r.commitMsgBuilder.Build(from, to, revs, emails, r.cfg.Contacts, canary, manualRollRequester)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	sklog.Infof("Creating new roll with commit message: \n%s", commitMsg)
	issueNum, err := r.rm.CreateNewRoll(ctx, from, to, revs, emails, dryRun, canary, commitMsg)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	issue := &autoroll.AutoRollIssue{
		AttemptStart: time.Now(),
		IsDryRun:     dryRun,
		Issue:        issueNum,
		Manual:       manualRollRequester != "",
		RollingFrom:  from.Id,
		RollingTo:    to.Id,
	}
	return issue, nil
}

// FailureThrottle returns a state_machine.Throttler indicating that we have
// failed to roll too many times within a time period.
func (r *AutoRoller) FailureThrottle() *state_machine.Throttler {
	return r.failureThrottle
}

// GetConfig implements state_machine.AutoRollerImpl.
func (r *AutoRoller) GetConfig() *config.Config {
	return r.cfg
}

// GetCurrentRev implements state_machine.AutoRollerImpl.
func (r *AutoRoller) GetCurrentRev() *revision.Revision {
	r.statusMtx.RLock()
	defer r.statusMtx.RUnlock()
	return r.lastRollRev
}

// GetNextRollRev implements state_machine.AutoRollerImpl.
func (r *AutoRoller) GetNextRollRev() *revision.Revision {
	r.statusMtx.RLock()
	defer r.statusMtx.RUnlock()
	return r.nextRollRev
}

// GetLastNRollRevs returns the revision IDs for up to N most recent rolls,
// sorted most recent first.
func (r *AutoRoller) GetLastNRollRevs(n int) []string {
	rolls := r.recent.GetRecentRolls()
	if len(rolls) > n {
		rolls = rolls[:n]
	}
	revs := make([]string, 0, n)
	for _, roll := range rolls {
		revs = append(revs, roll.RollingTo)
	}
	return revs
}

// InRollWindow implements state_machine.AutoRollerImpl.
func (r *AutoRoller) InRollWindow(t time.Time) bool {
	return r.timeWindow.Test(t)
}

// RolledPast implements state_machine.AutoRollerImpl.
func (r *AutoRoller) RolledPast(ctx context.Context, rev *revision.Revision) (bool, error) {
	r.statusMtx.RLock()
	defer r.statusMtx.RUnlock()
	// If we've rolled to this rev, then we're past it.
	if rev.Id == r.lastRollRev.Id {
		return true, nil
	}
	// If rev is the nextRollRev, then we aren't past it.
	if rev.Id == r.nextRollRev.Id {
		return false, nil
	}
	// If rev is the tipRev (and we haven't rolled to it), then we aren't
	// past it.
	if rev.Id == r.tipRev.Id {
		return false, nil
	}
	// If rev is any of the notRolledRevs, then we haven't rolled past it.
	for _, notRolledRev := range r.notRolledRevs {
		if rev.Id == notRolledRev.Id {
			return false, nil
		}
	}
	// We don't know about this rev. Assuming the revs we do know about are
	// valid, we must have rolled past this one.
	return true, nil
}

// GetRevisionsInRoll returns a list of revisions in a roll.
func (r *AutoRoller) GetRevisionsInRoll(ctx context.Context, roll state_machine.RollCLImpl) []*revision.Revision {
	revs, err := r.rm.LogRevisions(ctx, roll.RollingFrom(), roll.RollingTo())
	if err != nil {
		sklog.Errorf("Failed to retrieve revisions in roll %d: %s", roll.IssueID(), err)
		return nil
	}
	return revs
}

// SafetyThrottle returns a state_machine.Throttler indicating that we have
// attempted to upload too many CLs within a time period.
func (r *AutoRoller) SafetyThrottle() *state_machine.Throttler {
	return r.safetyThrottle
}

// SuccessThrottle returns a state_machine.Throttler indicating whether we have
// successfully rolled too many times within a time period.
func (r *AutoRoller) SuccessThrottle() *state_machine.Throttler {
	return r.successThrottle
}

// DryRunSuccessThrottle returns a state_machine.Throttler indicating whether we
// have successfully completed a dry run too recently.
func (r *AutoRoller) DryRunSuccessThrottle() *state_machine.Throttler {
	return r.dryRunSuccessThrottle
}

// UpdateRepos implements state_machine.AutoRollerImpl.
func (r *AutoRoller) UpdateRepos(ctx context.Context) error {
	lastRollRev, tipRev, notRolledRevs, err := r.rm.Update(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	r.strategyMtx.RLock()
	defer r.strategyMtx.RUnlock()
	nextRollRev := r.strategy.GetNextRollRev(notRolledRevs)
	if nextRollRev == nil {
		nextRollRev = lastRollRev
	}
	numValid := 0
	for _, rev := range notRolledRevs {
		if rev.InvalidReason == "" {
			numValid++
		}
	}
	sklog.Infof("lastRollRev is: %s", lastRollRev.Id)
	sklog.Infof("tipRev is:      %s", tipRev.Id)
	sklog.Infof("nextRollRev is: %s", nextRollRev.Id)
	sklog.Infof("notRolledRevs:  %d (%d valid roll candidates)", len(notRolledRevs), numValid)
	if numValid == 0 && lastRollRev.Id != nextRollRev.Id {
		var b strings.Builder
		for idx, rev := range notRolledRevs {
			if idx > 4 {
				b.WriteString("...\n")
				break
			}
			b.WriteString(fmt.Sprintf("%s: %s\n", rev.String(), rev.InvalidReason))
		}
		sklog.Warningf("Found no valid roll candidates! Example invalid revisions:\n%s", b.String())
	}

	// Sanity checks.
	foundLast := false
	foundTip := false
	foundNext := false
	for _, rev := range notRolledRevs {
		if rev.Id == lastRollRev.Id {
			foundLast = true
		}
		if rev.Id == tipRev.Id {
			foundTip = true
		}
		if rev.Id == nextRollRev.Id {
			foundNext = true
		}
	}
	if foundLast {
		return skerr.Fmt("Last roll rev %s found in not-rolled revs!", lastRollRev.Id)
	}
	if len(notRolledRevs) > 0 {
		if !foundTip {
			return skerr.Fmt("Tip rev %s not found in not-rolled revs!", tipRev.Id)
		}
		if !foundNext && nextRollRev.Id != lastRollRev.Id {
			return skerr.Fmt("Next roll rev %s not found in not-rolled revs!", nextRollRev.Id)
		}
		if nextRollRev.Id == lastRollRev.Id {
			if numValid == 0 {
				sklog.Warningf("There are revisions to roll, but the next roll rev %q equals the last roll rev; all %d not-yet-rolled revisions are invalid.", nextRollRev.Id, len(notRolledRevs))
			} else {
				return skerr.Fmt("There are revisions to roll, but the next roll rev %q equals the last roll rev; at least one revision is a valid roll candidate.", nextRollRev.Id)
			}
		}
	} else {
		if tipRev.Id != lastRollRev.Id {
			return skerr.Fmt("No revisions to roll, but tip rev %s does not equal last-rolled rev %s", tipRev.Id, lastRollRev.Id)
		}
		if nextRollRev.Id != lastRollRev.Id {
			return skerr.Fmt("No revisions to roll, but next roll rev is: %s", nextRollRev.Id)
		}
	}

	// Store the revs.
	r.statusMtx.Lock()
	defer r.statusMtx.Unlock()
	r.lastRollRev = lastRollRev
	r.nextRollRev = nextRollRev
	r.notRolledRevs = notRolledRevs
	r.tipRev = tipRev
	r.reportRevisionDetection(ctx)
	return nil
}

// Update the status information of the roller.
func (r *AutoRoller) updateStatus(ctx context.Context, replaceLastError bool, lastError string) error {
	r.statusMtx.Lock()
	defer r.statusMtx.Unlock()

	recent := r.recent.GetRecentRolls()

	if !replaceLastError {
		lastError = r.status.Get().Error
	}

	failureThrottledUntil := r.failureThrottle.ThrottledUntil().Unix()
	safetyThrottledUntil := r.safetyThrottle.ThrottledUntil().Unix()
	successThrottledUntil := r.successThrottle.ThrottledUntil().Unix()
	dryRunSuccessThrottledUntil := r.dryRunSuccessThrottle.ThrottledUntil().Unix()
	throttledUntil := failureThrottledUntil
	if safetyThrottledUntil > throttledUntil {
		throttledUntil = safetyThrottledUntil
	}
	if successThrottledUntil > throttledUntil {
		throttledUntil = successThrottledUntil
	}
	if dryRunSuccessThrottledUntil > throttledUntil {
		throttledUntil = dryRunSuccessThrottledUntil
	}

	notRolledRevs := r.notRolledRevs
	numNotRolled := len(notRolledRevs)
	sklog.Infof("Updating status (%d revisions behind)", numNotRolled)
	if numNotRolled > maxNotRolledRevs {
		notRolledRevs = notRolledRevs[:1]
		sklog.Warningf("Truncating NotRolledRevisions; %d is more than the maximum of %d", numNotRolled, maxNotRolledRevs)
	}
	currentRollRev := ""
	currentRoll := r.recent.CurrentRoll()
	if currentRoll != nil {
		currentRollRev = currentRoll.RollingTo
	}
	if err := r.status.Set(ctx, r.roller, &status.AutoRollStatus{
		AutoRollMiniStatus: status.AutoRollMiniStatus{
			CurrentRollRev:              currentRollRev,
			LastRollRev:                 r.lastRollRev.Id,
			Mode:                        r.GetMode(),
			NumFailedRolls:              r.recent.NumFailedRolls(),
			NumNotRolledCommits:         numNotRolled,
			Timestamp:                   time.Now().UTC(),
			LastSuccessfulRollTimestamp: r.recent.LastSuccessfulRollTime(),
		},
		ChildName:          r.cfg.ChildDisplayName,
		CurrentRoll:        currentRoll,
		Error:              lastError,
		FullHistoryUrl:     r.codereview.GetFullHistoryUrl(),
		IssueUrlBase:       r.codereview.GetIssueUrlBase(),
		LastRoll:           r.recent.LastRoll(),
		NotRolledRevisions: notRolledRevs,
		Recent:             recent,
		Status:             string(r.sm.Current()),
		ThrottledUntil:     throttledUntil,
		ValidModes:         modes.ValidModes,
		ValidStrategies:    r.cfg.ValidStrategies(),
	}); err != nil {
		return err
	}
	// Log the current reviewers(s).
	sklog.Infof("Current reviewers: %v", r.GetEmails())
	return r.status.Update(ctx)
}

// Tick runs one iteration of the roller.
func (r *AutoRoller) Tick(ctx context.Context) error {
	r.runningMtx.Lock()
	defer r.runningMtx.Unlock()

	sklog.Infof("Running autoroller.")

	// Cleanup and exit if requested.
	needsCleanup, err := roller_cleanup.NeedsCleanup(ctx, r.cleanup, r.roller)
	if err != nil {
		return skerr.Wrap(err)
	}
	if needsCleanup {
		sklog.Warningf("Deleting local data and exiting...")
		if err := r.DeleteLocalData(ctx); err != nil {
			return skerr.Wrap(err)
		}
		// TODO(borenet): Should we instead pass in a cancel function for the
		// top-level Context, so that things can shut down cleanly?
		os.Exit(0)
	}

	// Determine if we should unthrottle.
	shouldUnthrottle, err := r.throttle.Get(ctx, r.roller)
	if err != nil {
		return skerr.Wrapf(err, "Failed to determine whether we should unthrottle")
	}
	if shouldUnthrottle {
		if err := r.unthrottle(ctx); err != nil {
			return skerr.Wrapf(err, "Failed to unthrottle")
		}
		if err := r.throttle.Reset(ctx, r.roller); err != nil {
			return skerr.Wrapf(err, "Failed to reset unthrottle counter")
		}
	}

	// Update modes and strategies.
	if err := r.modeHistory.Update(ctx); err != nil {
		return skerr.Wrapf(err, "Failed to update mode history")
	}
	oldStrategy := r.strategyHistory.CurrentStrategy().Strategy
	if err := r.strategyHistory.Update(ctx); err != nil {
		return skerr.Wrapf(err, "Failed to update strategy history")
	}
	newStrategy := r.strategyHistory.CurrentStrategy().Strategy
	if oldStrategy != newStrategy {
		strat, err := strategy.GetNextRollStrategy(newStrategy)
		if err != nil {
			return skerr.Wrapf(err, "Failed to get next roll strategy")
		}
		r.strategyMtx.Lock()
		r.strategy = strat
		r.strategyMtx.Unlock()
	}

	// Run the state machine.
	lastErr := r.sm.NextTransitionSequence(ctx)
	lastErrStr := ""
	if lastErr != nil {
		lastErrStr = lastErr.Error()
	}

	// Update the status information.
	if err := r.updateStatus(ctx, true, lastErrStr); err != nil {
		return skerr.Wrapf(err, "Failed to update status")
	}
	sklog.Infof("Autoroller state %s", r.sm.Current())
	if lastRoll := r.recent.LastRoll(); lastRoll != nil && util.In(lastRoll.Result, []string{autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, autoroll.ROLL_RESULT_SUCCESS}) {
		r.liveness.ManualReset(lastRoll.Modified)
	}
	return skerr.Wrapf(lastErr, "Failed state transition sequence")
}

// AddComment adds a comment to the given roll CL.
func (r *AutoRoller) AddComment(ctx context.Context, issueNum int64, message, user string, timestamp time.Time) error {
	roll, err := r.recent.Get(ctx, issueNum)
	if err != nil {
		return skerr.Fmt("No such issue %d", issueNum)
	}
	id := fmt.Sprintf("%d_%d", issueNum, len(roll.Comments))
	roll.Comments = append(roll.Comments, comment.New(id, message, user))
	return r.recent.Update(ctx, roll)
}

// AddHandlers implements main.AutoRollerI.
func (r *AutoRoller) AddHandlers(chi.Router) {}

// Callback function which runs when roll CLs are closed.
func (r *AutoRoller) rollFinished(ctx context.Context, justFinished codereview.RollImpl) error {
	recent := r.recent.GetRecentRolls()
	// Sanity check: pop any rolls which occurred after the one which just
	// finished.
	idx := -1
	var currentRoll *autoroll.AutoRollIssue
	for i, roll := range recent {
		issue := fmt.Sprintf("%d", roll.Issue)
		if issue == justFinished.IssueID() {
			idx = i
			currentRoll = roll
			break
		}
	}
	if currentRoll == nil {
		return skerr.Fmt("Unable to find just-finished roll %q in recent list!", justFinished.IssueID())
	}

	// Feed AutoRoll stats into metrics.
	v := int64(0)
	if currentRoll.Closed && currentRoll.Committed {
		v = int64(1)
	}
	metrics2.GetInt64Metric("autoroll_last_roll_result", map[string]string{"roller": r.cfg.RollerName}).Update(v)

	recent = recent[idx:]
	var lastRoll *autoroll.AutoRollIssue
	if len(recent) > 1 {
		lastRoll = recent[1]
	} else {
		// If there are no other rolls, then the below alerts do not apply.
		return nil
	}

	issueURL := fmt.Sprintf("%s%d", r.codereview.GetIssueUrlBase(), currentRoll.Issue)

	// Send notifications if this roll had a different result from the last
	// roll, ie. success -> failure or failure -> success.
	currentSuccess := util.In(currentRoll.Result, autoroll.SUCCESS_RESULTS)
	if lastRoll != nil {
		lastSuccess := util.In(lastRoll.Result, autoroll.SUCCESS_RESULTS)
		if currentSuccess && !lastSuccess {
			r.notifier.SendNewSuccess(ctx, fmt.Sprintf("%d", currentRoll.Issue), issueURL)
		} else if !currentSuccess && lastSuccess {
			r.notifier.SendNewFailure(ctx, fmt.Sprintf("%d", currentRoll.Issue), issueURL)
		}
	}

	// Send a notification if the last N rolls failed in a row.
	nFailed := 0
	// recent is in reverse chronological order.
	for _, roll := range recent {
		if util.In(roll.Result, autoroll.SUCCESS_RESULTS) {
			break
		} else {
			nFailed++
		}
	}
	if nFailed == notifyIfLastNFailed {
		r.notifier.SendLastNFailed(ctx, notifyIfLastNFailed, issueURL)
	}

	return nil
}

// Handle manual roll requests.
func (r *AutoRoller) handleManualRolls(ctx context.Context) error {
	r.runningMtx.Lock()
	defer r.runningMtx.Unlock()

	if r.GetMode() == modes.ModeOffline {
		return nil
	}

	failManualRoll := func(req *manual.ManualRollRequest, err error) error {
		req.Status = manual.STATUS_COMPLETE
		req.Result = manual.RESULT_FAILURE
		req.ResultDetails = err.Error()
		sklog.Errorf("Failed to create manual roll: %s", req.ResultDetails)
		r.notifier.SendManualRollCreationFailed(ctx, req.Requester, req.Revision, err)
		if err := r.manualRollDB.Put(req); err != nil {
			return skerr.Wrapf(err, "Failed to update manual roll request")
		}
		return nil
	}

	sklog.Infof("Searching manual roll requests for %s", r.cfg.RollerName)
	reqs, err := r.manualRollDB.GetIncomplete(r.cfg.RollerName)
	if err != nil {
		return skerr.Wrapf(err, "Failed to get incomplete rolls")
	}
	sklog.Infof("Found %d requests.", len(reqs))
	for _, req := range reqs {
		var issue *autoroll.AutoRollIssue
		var to *revision.Revision
		if req.NoResolveRevision {
			to = &revision.Revision{Id: req.Revision}
		} else {
			to, err = r.getRevision(ctx, req.Revision)
			if err != nil {
				err := skerr.Wrapf(err, "failed to resolve revision %q", req.Revision)
				if err := failManualRoll(req, err); err != nil {
					return skerr.Wrap(err)
				}
				continue
			}
		}
		if req.ExternalChangeId != "" {
			to.ExternalChangeId = req.ExternalChangeId
		}
		from := r.GetCurrentRev()
		if req.Status == manual.STATUS_PENDING {
			// Avoid creating rolls to the current revision.
			if to.Id == from.Id {
				err := skerr.Fmt("Already at revision %q", from.Id)
				if err := failManualRoll(req, err); err != nil {
					return skerr.Wrap(err)
				}
				continue
			}

			// It is possible to request a manual roll to a CL which has not
			// been submitted. Ensure that the Canary bit is set in this case,
			// to ensure that the roller does not allow the roll to land.
			notSubmittedReason, err := r.isRevisionNotSubmitted(ctx, req, to)
			if err != nil {
				return skerr.Wrap(err)
			}
			if notSubmittedReason != "" && !req.Canary {
				err := skerr.Fmt("Revision %s is not submitted (%s), and Canary is not set", to.URL, notSubmittedReason)
				if err := failManualRoll(req, err); err != nil {
					return skerr.Wrap(err)
				}
				continue
			}

			emails := []string{}
			if !req.NoEmail {
				emails = r.GetEmails()
				if !util.In(req.Requester, emails) {
					emails = append(emails, req.Requester)
				}
			}
			sklog.Infof("Creating manual roll to %s as requested by %s...", req.Revision, req.Requester)

			issue, err = r.createNewRoll(ctx, from, to, emails, req.DryRun, req.Canary, req.Requester)
			if err != nil {
				err := skerr.Wrapf(err, "failed to create manual roll for %s", req.Id)
				if err := failManualRoll(req, err); err != nil {
					return skerr.Wrap(err)
				}
				continue
			}
		} else if req.Status == manual.STATUS_STARTED {
			split := strings.Split(req.Url, "/")
			i, err := strconv.Atoi(split[len(split)-1])
			if err != nil {
				return skerr.Wrapf(err, "Failed to parse issue number from %s for %s: %s", req.Url, req.Id, err)
			}
			issue = &autoroll.AutoRollIssue{
				AttemptStart: time.Now(),
				RollingTo:    req.Revision,
				IsDryRun:     req.DryRun,
				Issue:        int64(i),
				Manual:       true,
			}
		} else {
			sklog.Errorf("Found manual roll request %s in unknown status %q", req.Id, req.Status)
			continue
		}
		sklog.Infof("Getting status for manual roll # %d", issue.Issue)
		roll, err := r.retrieveRoll(ctx, issue, from, to)
		if err != nil {
			return skerr.Wrapf(err, "Failed to retrieve manual roll %s: %s", req.Id, err)
		}
		req.Status = manual.STATUS_STARTED
		req.Url = roll.IssueURL()

		if req.DryRun {
			if roll.IsDryRunFinished() {
				req.Status = manual.STATUS_COMPLETE
				if roll.IsDryRunSuccess() {
					req.Result = manual.RESULT_SUCCESS
				} else {
					req.Result = manual.RESULT_FAILURE
				}
			}
		} else if roll.IsFinished() {
			req.Status = manual.STATUS_COMPLETE
			if roll.IsSuccess() {
				req.Result = manual.RESULT_SUCCESS
			} else {
				req.Result = manual.RESULT_FAILURE
			}
		}
		if err := r.manualRollDB.Put(req); err != nil {
			return skerr.Wrapf(err, "Failed to update manual roll request")
		}
	}
	return nil
}

// isRevisionNotSubmitted attempts to determine whether the requested revision
// has been submitted. If not, it returns a string indicating the reason it came
// to that conclusion.
func (r *AutoRoller) isRevisionNotSubmitted(ctx context.Context, req *manual.ManualRollRequest, to *revision.Revision) (string, error) {
	// These two bits are set for canary requests, so we can skip the below if
	// they're set.
	if req.NoResolveRevision {
		return "NoResolveRevision is set", nil
	} else if req.Canary {
		return "Canary is set", nil
	}
	return r.rm.GetNotSubmittedReason(ctx, to)
}

// getRevision retrieves the Revision with the given ID, attempting to avoid
// network and/or subprocesses.
func (r *AutoRoller) getRevision(ctx context.Context, id string) (*revision.Revision, error) {
	if id == r.lastRollRev.Id {
		return r.lastRollRev, nil
	}
	if id == r.nextRollRev.Id {
		return r.nextRollRev, nil
	}
	if id == r.tipRev.Id {
		return r.tipRev, nil
	}
	for _, rev := range r.notRolledRevs {
		if id == rev.Id {
			return rev, nil
		}
	}
	return r.rm.GetRevision(ctx, id)
}

// reportRevisionDetection reports the time it took for revisions to be noticed
// by the roller.
func (r *AutoRoller) reportRevisionDetection(_ context.Context) {
	now := time.Now()

	// Remove older revisions from reportedRevs to stop reportedRevs from growing
	// indefinitely. To prevent double reporting if a roll is reverted, remove
	// once size exceeds 100. This length should be enough to cover all revisions
	// included in a reverted roll for most rollers.
	for id, timestamp := range r.reportedRevs {
		if len(r.reportedRevs) <= 100 {
			break
		}
		if timestamp.Before(r.lastRollRev.Timestamp) {
			delete(r.reportedRevs, id)
		}
	}

	m := metrics2.GetFloat64SummaryMetric(
		"autoroll_revision_detection_latency",
		map[string]string{"roller": r.cfg.RollerName})
	for _, rev := range r.notRolledRevs {
		if !r.reportedRevs[rev.Id].IsZero() || rev.Timestamp.IsZero() {
			continue
		}
		r.reportedRevs[rev.Id] = rev.Timestamp
		diff := now.Sub(rev.Timestamp)
		m.Observe(diff.Seconds())
	}
}

// RequestCleanup implements state_machine.AutoRollerImpl.
func (r *AutoRoller) RequestCleanup(ctx context.Context, reason string) error {
	return skerr.Wrap(r.cleanup.RequestCleanup(ctx, &roller_cleanup.CleanupRequest{
		RollerID:      r.roller,
		NeedsCleanup:  true,
		User:          r.roller,
		Timestamp:     time.Now(),
		Justification: reason,
	}))
}

// DeleteLocalData deletes the local data stored on this roller. The caller
// should exit immediately afterward, otherwise processes which depend on this
// local data will have problems.
func (r *AutoRoller) DeleteLocalData(ctx context.Context) error {
	// Delete the contents of r.workdir.  We cannot simply delete and recreate
	// the workdir because for most rollers the workdir is a mount which cannot
	// be removed.
	toRemove, err := os.ReadDir(r.workdir)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, item := range toRemove {
		if err := os.RemoveAll(filepath.Join(r.workdir, item.Name())); err != nil {
			return skerr.Wrap(err)
		}
	}
	// Clear the needs-cleanup bit.
	if err := r.cleanup.RequestCleanup(ctx, &roller_cleanup.CleanupRequest{
		RollerID:      r.roller,
		NeedsCleanup:  false,
		User:          r.roller,
		Timestamp:     now.Now(ctx),
		Justification: "Deleted local data",
	}); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
