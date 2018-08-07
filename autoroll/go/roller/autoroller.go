package roller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"go.skia.org/infra/autoroll/go/modes"
	arb_notifier "go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/comment"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// We'll send a notification if this many rolls fail in a row.
	NOTIFY_IF_LAST_N_FAILED = 3
)

// AutoRoller is a struct which automates the merging new revisions of one
// project into another.
type AutoRoller struct {
	childName       string
	cqExtraTrybots  []string
	currentRoll     RollImpl
	emails          []string
	emailsMtx       sync.RWMutex
	failureThrottle *state_machine.Throttler
	gerrit          *gerrit.Gerrit
	liveness        metrics2.Liveness
	modeHistory     *modes.ModeHistory
	notifier        *arb_notifier.AutoRollNotifier
	parentName      string
	recent          *recent_rolls.RecentRolls
	retrieveRoll    func(context.Context, *AutoRoller, int64) (RollImpl, error)
	rm              repo_manager.RepoManager
	roller          string
	runningMtx      sync.Mutex
	safetyThrottle  *state_machine.Throttler
	serverURL       string
	sheriff         []string
	sm              *state_machine.AutoRollStateMachine
	status          *status.AutoRollStatusCache
	statusMtx       sync.RWMutex
	strategyHistory *strategy.StrategyHistory
	successThrottle *state_machine.Throttler
	rollIntoAndroid bool
}

// NewAutoRoller returns an AutoRoller instance.
func NewAutoRoller(ctx context.Context, c AutoRollerConfig, emailer *email.GMail, g *gerrit.Gerrit, githubClient *github.GitHub, workdir, recipesCfgFile, serverURL, gitcookiesPath string, gcsClient gcs.GCSClient, rollerName string) (*AutoRoller, error) {
	// Validation and setup.
	if err := c.Validate(); err != nil {
		return nil, err
	}

	retrieveRoll := func(ctx context.Context, arb *AutoRoller, issue int64) (RollImpl, error) {
		return newGerritRoll(ctx, arb.gerrit, arb.rm, arb.recent, issue, arb.rollFinished)
	}

	// Create the RepoManager.
	var rm repo_manager.RepoManager
	var err error
	if c.AFDORepoManager != nil {
		rm, err = repo_manager.NewAFDORepoManager(ctx, c.AFDORepoManager, workdir, g, recipesCfgFile, serverURL, nil)
	} else if c.AndroidRepoManager != nil {
		retrieveRoll = func(ctx context.Context, arb *AutoRoller, issue int64) (RollImpl, error) {
			return newGerritAndroidRoll(ctx, arb.gerrit, arb.rm, arb.recent, issue, arb.rollFinished)
		}
		rm, err = repo_manager.NewAndroidRepoManager(ctx, c.AndroidRepoManager, workdir, g, serverURL)
	} else if c.CopyRepoManager != nil {
		rm, err = repo_manager.NewCopyRepoManager(ctx, c.CopyRepoManager, workdir, g, recipesCfgFile, serverURL)

	} else if c.DEPSRepoManager != nil {
		rm, err = repo_manager.NewDEPSRepoManager(ctx, c.DEPSRepoManager, workdir, g, recipesCfgFile, serverURL)
	} else if c.FuchsiaSDKRepoManager != nil {
		rm, err = repo_manager.NewFuchsiaSDKRepoManager(ctx, c.FuchsiaSDKRepoManager, workdir, g, recipesCfgFile, serverURL, nil)
	} else if c.GithubRepoManager != nil {
		rm, err = repo_manager.NewGithubRepoManager(ctx, c.GithubRepoManager, workdir, githubClient, recipesCfgFile, serverURL)
		retrieveRoll = func(ctx context.Context, arb *AutoRoller, pullRequestNum int64) (RollImpl, error) {
			return newGithubRoll(ctx, githubClient, arb.rm, arb.recent, pullRequestNum, arb.rollFinished)
		}
	} else if c.GithubDEPSRepoManager != nil {
		rm, err = repo_manager.NewGithubDEPSRepoManager(ctx, c.GithubDEPSRepoManager, workdir, githubClient, recipesCfgFile, serverURL)
		retrieveRoll = func(ctx context.Context, arb *AutoRoller, pullRequestNum int64) (RollImpl, error) {
			return newGithubRoll(ctx, githubClient, arb.rm, arb.recent, pullRequestNum, arb.rollFinished)
		}
	} else if c.ManifestRepoManager != nil {
		rm, err = repo_manager.NewManifestRepoManager(ctx, c.ManifestRepoManager, workdir, g, recipesCfgFile, serverURL)
	} else if c.NoCheckoutDEPSRepoManager != nil {
		rm, err = repo_manager.NewNoCheckoutDEPSRepoManager(ctx, c.NoCheckoutDEPSRepoManager, workdir, g, recipesCfgFile, serverURL, gitcookiesPath, nil)
	} else {
		return nil, errors.New("Invalid roller config; no repo manager defined!")
	}
	if err != nil {
		return nil, err
	}

	sh, err := strategy.NewStrategyHistory(ctx, rollerName, rm.DefaultStrategy(), rm.ValidStrategies())
	if err != nil {
		return nil, fmt.Errorf("Failed to create strategy history: %s", err)
	}
	initialStrategy := sh.CurrentStrategy().Strategy
	if err := repo_manager.SetStrategy(ctx, rm, initialStrategy); err != nil {
		return nil, fmt.Errorf("Failed to set repo manager strategy: %s", err)
	}
	if err := rm.Update(ctx); err != nil {
		return nil, fmt.Errorf("Failed initial repo manager update: %s", err)
	}

	recent, err := recent_rolls.NewRecentRolls(ctx, rollerName)
	if err != nil {
		return nil, fmt.Errorf("Failed to create recent rolls DB: %s", err)
	}

	mh, err := modes.NewModeHistory(ctx, rollerName)
	if err != nil {
		return nil, fmt.Errorf("Failed to create mode history: %s", err)
	}

	// Throttling counters.
	if c.SafetyThrottle == nil {
		c.SafetyThrottle = SAFETY_THROTTLE_CONFIG_DEFAULT
	}
	safetyThrottle, err := state_machine.NewThrottler(ctx, gcsClient, rollerName+"/attempt_counter", c.SafetyThrottle.TimeWindow, c.SafetyThrottle.AttemptCount)
	if err != nil {
		return nil, err
	}

	failureThrottle, err := state_machine.NewThrottler(ctx, gcsClient, rollerName+"/fail_counter", time.Hour, 1)
	if err != nil {
		return nil, err
	}

	maxRollFreq, err := human.ParseDuration(c.MaxRollFrequency)
	if err != nil {
		return nil, err
	}
	successThrottle, err := state_machine.NewThrottler(ctx, gcsClient, rollerName+"/success_counter", maxRollFreq, 1)
	if err != nil {
		return nil, err
	}

	emails, err := getSheriff(c.ParentName, c.ChildName, c.Sheriff)
	if err != nil {
		return nil, err
	}

	n := arb_notifier.New(c.ChildName, c.ParentName, emailer)
	if err := n.Router().AddFromConfigs(ctx, c.Notifiers); err != nil {
		return nil, err
	}
	statusCache, err := status.NewCache(ctx, rollerName)
	if err != nil {
		return nil, err
	}
	arb := &AutoRoller{
		childName:       c.ChildName,
		cqExtraTrybots:  c.CqExtraTrybots,
		emails:          emails,
		failureThrottle: failureThrottle,
		gerrit:          g,
		liveness:        metrics2.NewLiveness("last_autoroll_landed", map[string]string{"roller": c.RollerName()}),
		modeHistory:     mh,
		notifier:        n,
		parentName:      c.ParentName,
		recent:          recent,
		retrieveRoll:    retrieveRoll,
		rm:              rm,
		roller:          rollerName,
		safetyThrottle:  safetyThrottle,
		serverURL:       serverURL,
		sheriff:         c.Sheriff,
		status:          statusCache,
		strategyHistory: sh,
		successThrottle: successThrottle,
	}
	sm, err := state_machine.New(ctx, arb, n, gcsClient, rollerName)
	if err != nil {
		return nil, err
	}
	arb.sm = sm
	current := recent.CurrentRoll()
	if current != nil {
		roll, err := arb.retrieveRoll(ctx, arb, current.Issue)
		if err != nil {
			return nil, err
		}
		arb.currentRoll = roll
	}
	return arb, nil
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
	}
	sklog.Infof("Not a sync error.")
	return false
}

// Start initiates the AutoRoller's loop.
func (r *AutoRoller) Start(ctx context.Context, tickFrequency, repoFrequency time.Duration) {
	sklog.Infof("Starting autoroller.")
	repo_manager.Start(ctx, r.rm, repoFrequency)
	lv := metrics2.NewLiveness("last_successful_autoroll_tick")
	cleanup.Repeat(tickFrequency, func() {
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

	// Update the current sheriff in a loop.
	cleanup.Repeat(30*time.Minute, func() {
		emails, err := getSheriff(r.parentName, r.childName, r.sheriff)
		if err != nil {
			sklog.Errorf("Failed to retrieve current sheriff: %s", err)
		} else {
			r.emailsMtx.Lock()
			defer r.emailsMtx.Unlock()
			r.emails = emails
		}
	}, nil)
}

// See documentation for state_machine.AutoRollerImpl interface.
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

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) GetMode() string {
	return r.modeHistory.CurrentMode().Mode
}

// SetMode sets the desired mode of the bot.
func (r *AutoRoller) SetMode(ctx context.Context, mode, user, message string) error {
	if err := r.modeHistory.Add(ctx, mode, user, message); err != nil {
		return err
	}
	r.notifier.SendModeChange(ctx, user, mode, message)

	// Update the status so that the mode change shows up on the UI.
	return r.updateStatus(ctx, false, "")
}

// SetStrategy sets the desired next-roll-revision strategy for the roller.
func (r *AutoRoller) SetStrategy(ctx context.Context, strategy, user, message string) error {
	if err := repo_manager.SetStrategy(ctx, r.rm, strategy); err != nil {
		return err
	}
	if err := r.strategyHistory.Add(ctx, strategy, user, message); err != nil {
		return err
	}
	r.notifier.SendStrategyChange(ctx, user, strategy, message)

	// Update the status so that the strategy change shows up on the UI.
	return r.updateStatus(ctx, false, "")
}

// Return the roll-up status of the bot.
func (r *AutoRoller) GetStatus(includeError bool) *status.AutoRollStatus {
	r.statusMtx.RLock()
	defer r.statusMtx.RUnlock()
	status := r.status.Get()
	if !includeError {
		status.Error = ""
	}
	return status
}

// Return minimal status information for the bot.
func (r *AutoRoller) GetMiniStatus() *status.AutoRollMiniStatus {
	r.statusMtx.RLock()
	defer r.statusMtx.RUnlock()
	return r.status.GetMini()
}

// Return the AutoRoll user.
func (r *AutoRoller) GetUser() string {
	return r.rm.User()
}

// Reset all of the roller's throttle timers.
func (r *AutoRoller) Unthrottle(ctx context.Context) error {
	if err := r.failureThrottle.Reset(ctx); err != nil {
		return err
	}
	if err := r.safetyThrottle.Reset(ctx); err != nil {
		return err
	}
	if err := r.successThrottle.Reset(ctx); err != nil {
		return err
	}
	return nil
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) UploadNewRoll(ctx context.Context, from, to string, dryRun bool) error {
	issueNum, err := r.rm.CreateNewRoll(ctx, from, to, r.GetEmails(), strings.Join(r.cqExtraTrybots, ";"), dryRun)
	if err != nil {
		return err
	}
	roll, err := r.retrieveRoll(ctx, r, issueNum)
	if err != nil {
		return err
	}
	if err := roll.InsertIntoDB(ctx); err != nil {
		return err
	}
	r.currentRoll = roll
	return nil
}

// Return a state_machine.Throttler indicating that we have failed to roll too many
// times within a time period.
func (r *AutoRoller) FailureThrottle() *state_machine.Throttler {
	return r.failureThrottle
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) GetCurrentRev() string {
	return r.rm.LastRollRev()
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) GetNextRollRev() string {
	return r.rm.NextRollRev()
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) RolledPast(ctx context.Context, rev string) (bool, error) {
	return r.rm.RolledPast(ctx, rev)
}

// Return a state_machine.Throttler indicating that we have attempted to upload too
// many CLs within a time period.
func (r *AutoRoller) SafetyThrottle() *state_machine.Throttler {
	return r.safetyThrottle
}

// Return a state_machine.Throttler indicating whether we have successfully rolled too
// many times within a time period.
func (r *AutoRoller) SuccessThrottle() *state_machine.Throttler {
	return r.successThrottle
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) UpdateRepos(ctx context.Context) error {
	return r.rm.Update(ctx)
}

// Update the status information of the roller.
func (r *AutoRoller) updateStatus(ctx context.Context, replaceLastError bool, lastError string) error {
	r.statusMtx.Lock()
	defer r.statusMtx.Unlock()

	recent := r.recent.GetRecentRolls()
	numFailures := 0
	for _, roll := range recent {
		if roll.Failed() {
			numFailures++
		} else if roll.Succeeded() {
			break
		}
	}
	if !replaceLastError {
		lastError = r.status.Get().Error
	}

	failureThrottledUntil := r.failureThrottle.ThrottledUntil().Unix()
	safetyThrottledUntil := r.safetyThrottle.ThrottledUntil().Unix()
	successThrottledUntil := r.successThrottle.ThrottledUntil().Unix()
	throttledUntil := failureThrottledUntil
	if safetyThrottledUntil > throttledUntil {
		throttledUntil = safetyThrottledUntil
	}
	if successThrottledUntil > throttledUntil {
		throttledUntil = successThrottledUntil
	}

	sklog.Infof("Updating status (%d)", r.rm.CommitsNotRolled())
	if err := status.Set(ctx, r.roller, &status.AutoRollStatus{
		AutoRollMiniStatus: status.AutoRollMiniStatus{
			NumFailedRolls:      numFailures,
			NumNotRolledCommits: r.rm.CommitsNotRolled(),
		},
		CurrentRoll:     r.recent.CurrentRoll(),
		Error:           lastError,
		FullHistoryUrl:  r.rm.GetFullHistoryUrl(),
		IssueUrlBase:    r.rm.GetIssueUrlBase(),
		LastRoll:        r.recent.LastRoll(),
		LastRollRev:     r.rm.LastRollRev(),
		Mode:            r.modeHistory.CurrentMode(),
		Recent:          recent,
		Status:          string(r.sm.Current()),
		Strategy:        r.strategyHistory.CurrentStrategy(),
		ThrottledUntil:  throttledUntil,
		ValidModes:      modes.VALID_MODES,
		ValidStrategies: r.rm.ValidStrategies(),
	}); err != nil {
		return err
	}
	return r.status.Update(ctx)
}

// Run one iteration of the roller.
func (r *AutoRoller) Tick(ctx context.Context) error {
	r.runningMtx.Lock()
	defer r.runningMtx.Unlock()

	sklog.Infof("Running autoroller.")
	// Run the state machine.
	lastErr := r.sm.NextTransitionSequence(ctx)
	lastErrStr := ""
	if lastErr != nil {
		lastErrStr = lastErr.Error()
	}

	// Update the status information.
	if err := r.updateStatus(ctx, true, lastErrStr); err != nil {
		return err
	}
	sklog.Infof("Autoroller state %s", r.sm.Current())
	if lastRoll := r.recent.LastRoll(); lastRoll != nil && util.In(lastRoll.Result, []string{autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, autoroll.ROLL_RESULT_SUCCESS}) {
		r.liveness.ManualReset(lastRoll.Modified)
	}
	return lastErr
}

// Add a comment to the given roll CL.
func (r *AutoRoller) AddComment(ctx context.Context, issueNum int64, message, user string, timestamp time.Time) error {
	roll, err := r.recent.Get(ctx, issueNum)
	if err != nil {
		return fmt.Errorf("No such issue %d", issueNum)
	}
	id := fmt.Sprintf("%d_%d", issueNum, len(roll.Comments))
	roll.Comments = append(roll.Comments, comment.New(id, message, user))
	return r.recent.Update(ctx, roll)
}

// Required for main.AutoRollerI. No specific HTTP handlers.
func (r *AutoRoller) AddHandlers(*mux.Router) {}

// Callback function which runs when roll CLs are closed.
func (r *AutoRoller) rollFinished(ctx context.Context, justFinished RollImpl) error {
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
		return fmt.Errorf("Unable to find just-finished roll %q in recent list!", justFinished.IssueID())
	}
	recent = recent[idx:]
	var lastRoll *autoroll.AutoRollIssue
	if len(recent) > 1 {
		lastRoll = recent[1]
	} else {
		// If there are no other rolls, then the below alerts do not apply.
		return nil
	}

	issueURL := fmt.Sprintf("%s%d", r.rm.GetIssueUrlBase(), currentRoll.Issue)

	// Send notifications if this roll had a different result from the last
	// roll, ie. success -> failure or failure -> success.
	currentSuccess := util.In(currentRoll.Result, autoroll.SUCCESS_RESULTS)
	lastSuccess := util.In(lastRoll.Result, autoroll.SUCCESS_RESULTS)
	if lastRoll != nil {
		if currentSuccess && !lastSuccess {
			r.notifier.SendNewSuccess(ctx, fmt.Sprintf("%d", currentRoll.Issue), issueURL)
		} else if !currentSuccess && lastSuccess {
			r.notifier.SendNewFailure(ctx, fmt.Sprintf("%d", currentRoll.Issue), issueURL)
		}
	}

	// Send a notification if the last N rolls failed in a row.
	lastNFailed := false
	if len(recent) >= NOTIFY_IF_LAST_N_FAILED {
		lastNFailed = true
		for _, roll := range recent[:NOTIFY_IF_LAST_N_FAILED] {
			if util.In(roll.Result, autoroll.SUCCESS_RESULTS) {
				lastNFailed = false
				break
			}
		}
	}
	if lastNFailed {
		r.notifier.SendLastNFailed(ctx, NOTIFY_IF_LAST_N_FAILED, issueURL)
	}

	return nil
}
