package roller

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/comment"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// AutoRoller is a struct which automates the merging new revisions of one
// project into another.
type AutoRoller struct {
	cqExtraTrybots  string
	currentRoll     RollImpl
	emails          []string
	emailsMtx       sync.RWMutex
	gerrit          *gerrit.Gerrit
	lastError       error
	liveness        metrics2.Liveness
	modeHistory     *modes.ModeHistory
	recent          *recent_rolls.RecentRolls
	retrieveRoll    func(*AutoRoller, int64) (RollImpl, error)
	rm              repo_manager.RepoManager
	runningMtx      sync.Mutex
	sm              *state_machine.AutoRollStateMachine
	status          *AutoRollStatusCache
	rollIntoAndroid bool
}

// newAutoRoller returns an AutoRoller instance.
func newAutoRoller(workdir, childPath, cqExtraTrybots string, emails []string, gerrit *gerrit.Gerrit, rm repo_manager.RepoManager, retrieveRoll func(*AutoRoller, int64) (RollImpl, error), tc *state_machine.ThrottleConfig) (*AutoRoller, error) {
	recent, err := recent_rolls.NewRecentRolls(path.Join(workdir, "recent_rolls.db"))
	if err != nil {
		return nil, err
	}

	mh, err := modes.NewModeHistory(path.Join(workdir, "autoroll_modes.db"))
	if err != nil {
		return nil, err
	}

	arb := &AutoRoller{
		cqExtraTrybots: cqExtraTrybots,
		emails:         emails,
		gerrit:         gerrit,
		liveness:       metrics2.NewLiveness("last_autoroll_landed", map[string]string{"child_path": childPath}),
		modeHistory:    mh,
		recent:         recent,
		retrieveRoll:   retrieveRoll,
		rm:             rm,
		status:         &AutoRollStatusCache{},
	}
	sm, err := state_machine.New(arb, workdir, nil)
	if err != nil {
		return nil, err
	}
	arb.sm = sm
	current := recent.CurrentRoll()
	if current != nil {
		roll, err := arb.retrieveRoll(arb, current.Issue)
		if err != nil {
			return nil, err
		}
		arb.currentRoll = roll
	}
	return arb, nil
}

// NewAndroidAutoRoller returns an AutoRoller instance which rolls into Android.
func NewAndroidAutoRoller(workdir, parentBranch, childPath, childBranch, cqExtraTrybots string, emails []string, gerrit *gerrit.Gerrit, strategy repo_manager.NextRollStrategy, preUploadSteps []string, serverURL string, tc *state_machine.ThrottleConfig) (*AutoRoller, error) {
	rm, err := repo_manager.NewAndroidRepoManager(workdir, parentBranch, childPath, childBranch, gerrit, strategy, preUploadSteps, serverURL)
	if err != nil {
		return nil, err
	}
	retrieveRoll := func(arb *AutoRoller, issue int64) (RollImpl, error) {
		return newGerritAndroidRoll(arb.gerrit, arb.rm, arb.recent, issue)
	}
	return newAutoRoller(workdir, childPath, cqExtraTrybots, emails, gerrit, rm, retrieveRoll, tc)
}

// NewDEPSAutoRoller returns an AutoRoller instance which rolls using DEPS.
func NewDEPSAutoRoller(workdir, parentRepo, parentBranch, childPath, childBranch, cqExtraTrybots string, emails []string, gerrit *gerrit.Gerrit, depot_tools string, strategy repo_manager.NextRollStrategy, preUploadSteps []string, includeLog bool, depsCustomVars []string, serverURL string, tc *state_machine.ThrottleConfig) (*AutoRoller, error) {
	rm, err := repo_manager.NewDEPSRepoManager(workdir, parentRepo, parentBranch, childPath, childBranch, depot_tools, gerrit, strategy, preUploadSteps, includeLog, depsCustomVars, serverURL)
	if err != nil {
		return nil, err
	}
	retrieveRoll := func(arb *AutoRoller, issue int64) (RollImpl, error) {
		return newGerritRoll(arb.gerrit, arb.rm, arb.recent, issue)
	}
	return newAutoRoller(workdir, childPath, cqExtraTrybots, emails, gerrit, rm, retrieveRoll, tc)
}

// NewManifestAutoRoller returns an AutoRoller instance which rolls using DEPS.
func NewManifestAutoRoller(workdir, parentRepo, parentBranch, childPath, childBranch, cqExtraTrybots string, emails []string, gerrit *gerrit.Gerrit, depot_tools string, strategy repo_manager.NextRollStrategy, preUploadSteps []string, serverURL string, tc *state_machine.ThrottleConfig) (*AutoRoller, error) {
	rm, err := repo_manager.NewManifestRepoManager(workdir, parentRepo, parentBranch, childPath, childBranch, depot_tools, gerrit, strategy, preUploadSteps, serverURL)
	if err != nil {
		return nil, err
	}
	retrieveRoll := func(arb *AutoRoller, issue int64) (RollImpl, error) {
		return newGerritRoll(arb.gerrit, arb.rm, arb.recent, issue)
	}
	return newAutoRoller(workdir, childPath, cqExtraTrybots, emails, gerrit, rm, retrieveRoll, tc)
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
func (r *AutoRoller) Start(tickFrequency, repoFrequency time.Duration) {
	sklog.Infof("Starting autoroller.")
	repo_manager.Start(r.rm, repoFrequency)
	lv := metrics2.NewLiveness("last_successful_autoroll_tick")
	cleanup.Repeat(tickFrequency, func() {
		if err := r.Tick(); err != nil {
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
	}, func() {
		util.LogErr(r.recent.Close())
		util.LogErr(r.modeHistory.Close())
	})
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

// SetEmails sets the list of email addresses which are copied on rolls.
func (r *AutoRoller) SetEmails(e []string) {
	r.emailsMtx.Lock()
	defer r.emailsMtx.Unlock()
	emails := make([]string, len(e))
	copy(emails, e)
	r.emails = emails
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) GetMode() string {
	return r.modeHistory.CurrentMode().Mode
}

// SetMode sets the desired mode of the bot. This forces the bot to run and
// blocks until it finishes.
func (r *AutoRoller) SetMode(m, user, message string) error {
	if err := r.modeHistory.Add(m, user, message); err != nil {
		return err
	}
	return r.Tick()
}

// Return the roll-up status of the bot.
func (r *AutoRoller) GetStatus(includeError bool) *AutoRollStatus {
	return r.status.Get(includeError, nil)
}

// Return minimal status information for the bot.
func (r *AutoRoller) GetMiniStatus() *AutoRollMiniStatus {
	return r.status.GetMini()
}

// Return the AutoRoll user.
func (r *AutoRoller) GetUser() string {
	return r.rm.User()
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) UploadNewRoll(from, to string, dryRun bool) error {
	issueNum, err := r.rm.CreateNewRoll(from, to, r.GetEmails(), r.cqExtraTrybots, dryRun)
	if err != nil {
		return err
	}
	roll, err := r.retrieveRoll(r, issueNum)
	if err != nil {
		return err
	}
	if err := roll.InsertIntoDB(); err != nil {
		return err
	}
	r.currentRoll = roll
	return nil
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
func (r *AutoRoller) RolledPast(rev string) (bool, error) {
	return r.rm.RolledPast(rev)
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) UpdateRepos() error {
	return r.rm.Update()
}

// Run one iteration of the roller.
func (r *AutoRoller) Tick() error {
	r.runningMtx.Lock()
	defer r.runningMtx.Unlock()

	sklog.Infof("Running autoroller.")
	// Run the state machine.
	lastErr := r.sm.NextTransitionSequence()

	// Update the status information.
	lastErrorStr := ""
	if lastErr != nil {
		lastErrorStr = lastErr.Error()
	}
	recent := r.recent.GetRecentRolls()
	numFailures := 0
	for _, roll := range recent {
		if roll.Failed() {
			numFailures++
		} else if roll.Succeeded() {
			break
		}
	}
	sklog.Infof("Updating status (%d)", r.rm.CommitsNotRolled())
	if err := r.status.Set(&AutoRollStatus{
		AutoRollMiniStatus: AutoRollMiniStatus{
			NumFailedRolls:      numFailures,
			NumNotRolledCommits: r.rm.CommitsNotRolled(),
		},
		CurrentRoll:    r.recent.CurrentRoll(),
		Error:          lastErrorStr,
		FullHistoryUrl: r.gerrit.Url(0) + "/q/owner:" + r.GetUser(),
		IssueUrlBase:   r.gerrit.Url(0) + "/c/",
		LastRoll:       r.recent.LastRoll(),
		LastRollRev:    r.rm.LastRollRev(),
		Mode:           r.modeHistory.CurrentMode(),
		Recent:         recent,
		Status:         string(r.sm.Current()),
	}); err != nil {
		return err
	}
	sklog.Infof("Autoroller state %s", r.sm.Current())
	if lastRoll := r.recent.LastRoll(); lastRoll != nil && util.In(lastRoll.Result, []string{autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, autoroll.ROLL_RESULT_SUCCESS}) {
		r.liveness.ManualReset(lastRoll.Modified)
	}
	return lastErr
}

// Add a comment to the given roll CL.
func (r *AutoRoller) AddComment(issueNum int64, message, user string, timestamp time.Time) error {
	roll, err := r.recent.Get(issueNum)
	if err != nil {
		return fmt.Errorf("No such issue %d", issueNum)
	}
	id := fmt.Sprintf("%d_%d", issueNum, len(roll.Comments))
	roll.Comments = append(roll.Comments, comment.New(id, message, user))
	return r.recent.Update(roll)
}

// Required for main.AutoRollerI. No specific HTTP handlers.
func (r *AutoRoller) AddHandlers(*mux.Router) {}
