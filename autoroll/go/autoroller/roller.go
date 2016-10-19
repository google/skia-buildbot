package autoroller

import (
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/autoroll/go/autoroll_modes"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/util"
)

const (
	ROLL_ATTEMPT_THROTTLE_TIME = 10 * time.Minute
	ROLL_ATTEMPT_THROTTLE_NUM  = 3

	STATUS_DRY_RUN_FAILURE     = "dry run failed"
	STATUS_DRY_RUN_IN_PROGRESS = "dry run in progress"
	STATUS_DRY_RUN_SUCCESS     = "dry run succeeded"
	STATUS_ERROR               = "error"
	STATUS_IN_PROGRESS         = "in progress"
	STATUS_STOPPED             = "stopped"
	STATUS_THROTTLED           = "throttled"
	STATUS_UP_TO_DATE          = "up to date"
)

var (
	VALID_STATUSES = []string{
		STATUS_DRY_RUN_FAILURE,
		STATUS_DRY_RUN_IN_PROGRESS,
		STATUS_DRY_RUN_SUCCESS,
		STATUS_ERROR,
		STATUS_IN_PROGRESS,
		STATUS_STOPPED,
		STATUS_THROTTLED,
		STATUS_UP_TO_DATE,
	}
)

// AutoRoller is a struct used for managing DEPS rolls.
type AutoRoller struct {
	attemptCounter   *util.AutoDecrementCounter
	cqExtraTrybots   string
	emails           []string
	includeCommitLog bool
	emailMtx         sync.RWMutex
	lastError        error
	liveness         *metrics2.Liveness
	modeHistory      *autoroll_modes.ModeHistory
	modeMtx          sync.Mutex
	mtx              sync.RWMutex
	recent           *recent_rolls.RecentRolls
	rm               repo_manager.RepoManager
	rietveld         *rietveld.Rietveld
	runningMtx       sync.Mutex
	status           *autoRollStatusCache
}

// NewAutoRoller creates and returns a new AutoRoller which runs at the given frequency.
func NewAutoRoller(workdir, childPath, cqExtraTrybots string, emails []string, rietveld *rietveld.Rietveld, tickFrequency, repoFrequency time.Duration, depot_tools string) (*AutoRoller, error) {
	rm, err := repo_manager.NewRepoManager(workdir, childPath, repoFrequency, depot_tools)
	if err != nil {
		return nil, err
	}

	recent, err := recent_rolls.NewRecentRolls(path.Join(workdir, "recent_rolls.db"))
	if err != nil {
		return nil, err
	}

	mh, err := autoroll_modes.NewModeHistory(path.Join(workdir, "autoroll_modes.db"))
	if err != nil {
		return nil, err
	}

	arb := &AutoRoller{
		attemptCounter:   util.NewAutoDecrementCounter(ROLL_ATTEMPT_THROTTLE_TIME),
		cqExtraTrybots:   cqExtraTrybots,
		emails:           emails,
		includeCommitLog: true,
		liveness:         metrics2.NewLiveness("last-autoroll-landed", map[string]string{"child-path": childPath}),
		modeHistory:      mh,
		recent:           recent,
		rietveld:         rietveld,
		rm:               rm,
		status:           &autoRollStatusCache{},
	}

	// Cycle once to fill out the current status.
	if err := arb.doAutoRoll(); err != nil {
		return nil, err
	}

	go func() {
		for _ = range time.Tick(tickFrequency) {
			util.LogErr(arb.doAutoRoll())
		}
	}()

	return arb, nil
}

// Close closes all sub-structs of the AutoRoller.
func (r *AutoRoller) Close() error {
	err1 := r.recent.Close()
	err2 := r.modeHistory.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

// AutoRollStatus is a struct which provides roll-up status information about
// the AutoRoll Bot.
type AutoRollStatus struct {
	CurrentRoll *autoroll.AutoRollIssue   `json:"currentRoll"`
	Error       string                    `json:"error"`
	LastRoll    *autoroll.AutoRollIssue   `json:"lastRoll"`
	LastRollRev string                    `json:"lastRollRev"`
	Mode        string                    `json:"mode"`
	Recent      []*autoroll.AutoRollIssue `json:"recent"`
	Status      string                    `json:"status"`
	ValidModes  []string                  `json:"validModes"`
}

// autoRollStatusCache is a struct used for caching roll-up status
// information about the AutoRoll Bot.
type autoRollStatusCache struct {
	currentRoll *autoroll.AutoRollIssue
	lastError   string
	lastRoll    *autoroll.AutoRollIssue
	lastRollRev string
	mode        string
	mtx         sync.RWMutex
	recent      []*autoroll.AutoRollIssue
	status      string
}

// Get returns the current status information.
func (c *autoRollStatusCache) Get(includeError bool) *AutoRollStatus {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	recent := make([]*autoroll.AutoRollIssue, 0, len(c.recent))
	for _, r := range c.recent {
		recent = append(recent, r.Copy())
	}
	validModes := make([]string, len(autoroll_modes.VALID_MODES))
	copy(validModes, autoroll_modes.VALID_MODES)
	s := &AutoRollStatus{
		LastRollRev: c.lastRollRev,
		Mode:        c.mode,
		Recent:      recent,
		Status:      c.status,
		ValidModes:  validModes,
	}
	if c.currentRoll != nil {
		s.CurrentRoll = c.currentRoll.Copy()
	}
	if c.lastRoll != nil {
		s.LastRoll = c.lastRoll.Copy()
	}
	if includeError && c.lastError != "" {
		s.Error = c.lastError
	}
	return s
}

// set sets the current status information.
func (c *autoRollStatusCache) set(s *AutoRollStatus) error {
	if !util.In(string(s.Status), VALID_STATUSES) {
		return fmt.Errorf("Invalid status: %s", s.Status)
	}
	if s.Status == STATUS_ERROR {
		if s.Error == "" {
			return fmt.Errorf("Cannot set error status without an error!")
		}
	} else if s.Error != "" {
		return fmt.Errorf("Cannot be in any status other than error when an error occurred.")
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	recent := make([]*autoroll.AutoRollIssue, 0, len(s.Recent))
	for _, r := range s.Recent {
		recent = append(recent, r.Copy())
	}
	c.currentRoll = nil
	if s.CurrentRoll != nil {
		c.currentRoll = s.CurrentRoll.Copy()
	}
	c.lastRoll = nil
	if s.LastRoll != nil {
		c.lastRoll = s.LastRoll.Copy()
	}
	c.lastRollRev = s.LastRollRev
	c.mode = s.Mode
	c.recent = recent
	c.status = s.Status

	return nil
}

// GetStatus returns the roll-up status of the bot.
func (r *AutoRoller) GetStatus(includeError bool) *AutoRollStatus {
	return r.status.Get(includeError)
}

// SetMode sets the desired mode of the bot. This forces the bot to run and
// blocks until it finishes.
func (r *AutoRoller) SetMode(m, user, message string) error {
	r.modeMtx.Lock()
	defer r.modeMtx.Unlock()
	if err := r.modeHistory.Add(m, user, message); err != nil {
		return err
	}
	return r.doAutoRoll()
}

// isMode determines whether the bot is in the given mode.
func (r *AutoRoller) isMode(s string) bool {
	return r.modeHistory.CurrentMode() == s
}

// GetEmails returns the list of email addresses which are copied on DEPS rolls.
func (r *AutoRoller) GetEmails() []string {
	r.emailMtx.RLock()
	defer r.emailMtx.RUnlock()
	rv := make([]string, len(r.emails))
	copy(rv, r.emails)
	return rv
}

// SetEmails sets the list of email addresses which are copied on DEPS rolls.
func (r *AutoRoller) SetEmails(e []string) {
	r.emailMtx.Lock()
	defer r.emailMtx.Unlock()
	emails := make([]string, len(e))
	copy(emails, e)
	r.emails = emails
}

// closeIssue closes the given issue with the given message.
func (r *AutoRoller) closeIssue(issue *autoroll.AutoRollIssue, result, msg string) error {
	glog.Infof("Closing issue %d (result %q) with message: %s", issue.Issue, result, msg)
	if err := r.rietveld.Close(issue.Issue, msg); err != nil {
		return err
	}
	issue.Result = result
	issue.Closed = true
	issue.CommitQueue = false
	issue.CommitQueueDryRun = false
	return r.recent.Update(issue)
}

// addIssueComment adds a comment to the given issue.
func (r *AutoRoller) addIssueComment(issue *autoroll.AutoRollIssue, msg string) error {
	glog.Infof("Adding comment to issue: %q", msg)
	if err := r.rietveld.AddComment(issue.Issue, msg); err != nil {
		return err
	}
	updated, err := r.retrieveRoll(issue.Issue)
	if err != nil {
		return err
	}
	return r.recent.Update(updated)
}

// setDryRun sets the CQ dry run bit on the issue.
func (r *AutoRoller) setDryRun(issue *autoroll.AutoRollIssue, dryRun bool) error {
	// Unset the CQ and dry-run bits.
	props := map[string]string{
		"cq_dry_run": "0",
		"commit":     "0",
	}
	patchset := issue.Patchsets[len(issue.Patchsets)-1]
	if err := r.rietveld.SetProperties(issue.Issue, patchset, props); err != nil {
		return err
	}

	// Set the CQ and, if desired, the CQ dry run bit.
	props = map[string]string{
		"commit": "1",
	}
	if dryRun {
		props["cq_dry_run"] = "1"
	}
	if err := r.rietveld.SetProperties(issue.Issue, patchset, props); err != nil {
		return err
	}
	updated, err := r.retrieveRoll(issue.Issue)
	if err != nil {
		return err
	}
	return r.recent.Update(updated)
}

// updateCurrentRoll retrieves updated information about the current DEPS roll.
func (r *AutoRoller) updateCurrentRoll() error {
	currentRoll := r.recent.CurrentRoll()
	if currentRoll == nil {
		return nil
	}
	currentResult := currentRoll.Result

	updated, err := r.retrieveRoll(currentRoll.Issue)
	if err != nil {
		return err
	}

	// We have to rely on data we store for the dry run case.
	if !updated.Closed && util.In(currentResult, autoroll.DRY_RUN_RESULTS) {
		updated.Result = currentResult
	}

	// If the current roll succeeded, we need to make sure we update the
	// repo so that we see the roll commit. This can take some time, so
	// we have to repeatedly update until we see the commit.
	if updated.Committed {
		glog.Infof("Roll succeeded (%d); syncing the repo until it lands.", currentRoll.Issue)
		for {
			glog.Info("Syncing...")
			if err := r.rm.ForceUpdate(); err != nil {
				return err
			}
			if r.rm.RolledPast(currentRoll.RollingTo) {
				break
			}
			time.Sleep(10 * time.Second)
		}
		r.liveness.Reset()
	}
	return r.recent.Update(updated)
}

// retrieveRoll obtains the given DEPS roll from Rietveld.
func (r *AutoRoller) retrieveRoll(issueNum int64) (*autoroll.AutoRollIssue, error) {
	issue, err := r.rietveld.GetIssueProperties(issueNum, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to get issue properties: %s", err)
	}
	a, err := autoroll.FromRietveldIssue(issue, r.rm.FullChildHash)
	if err != nil {
		return nil, fmt.Errorf("Failed to convert issue format: %s", err)
	}
	tryResults, err := autoroll.GetTryResults(r.rietveld, a)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve try results: %s", err)
	}
	a.TryResults = tryResults
	return a, nil
}

// doAutoRoll is the primary method of the AutoRoll Bot. It runs on a timer,
// updates checkouts, manages active roll CLs, and uploads new rolls. It sets
// the status of the bot which may be read by users.
func (r *AutoRoller) doAutoRoll() error {
	status, lastError := r.doAutoRollInner()

	lastErrorStr := ""
	if lastError != nil {
		lastErrorStr = lastError.Error()
	}

	// Update status information.
	if err := r.status.set(&AutoRollStatus{
		CurrentRoll: r.recent.CurrentRoll(),
		Error:       lastErrorStr,
		LastRoll:    r.recent.LastRoll(),
		LastRollRev: r.rm.LastRollRev(),
		Mode:        r.modeHistory.CurrentMode(),
		Recent:      r.recent.GetRecentRolls(),
		Status:      status,
	}); err != nil {
		return err
	}

	return lastError
}

// makeRollResult determines what the result of a roll should be, given that
// it is going to be closed.
func (r *AutoRoller) makeRollResult(roll *autoroll.AutoRollIssue) string {
	if util.In(roll.Result, autoroll.DRY_RUN_RESULTS) {
		if roll.Result == autoroll.ROLL_RESULT_DRY_RUN_IN_PROGRESS {
			return autoroll.ROLL_RESULT_DRY_RUN_FAILURE
		} else {
			return roll.Result
		}
	}
	return autoroll.ROLL_RESULT_FAILURE
}

// doAutoRollInner does the actual work of the AutoRoll.
func (r *AutoRoller) doAutoRollInner() (string, error) {
	r.runningMtx.Lock()
	defer r.runningMtx.Unlock()

	// Get updated info about the current roll.
	if err := r.updateCurrentRoll(); err != nil {
		return STATUS_ERROR, err
	}

	// There's a currently-active roll. Determine whether or not it's still good.
	// If so, leave it open and exit. If not, close it so that we can open another.
	currentRoll := r.recent.CurrentRoll()
	if currentRoll != nil {
		glog.Infof("Found current roll: https://codereview.chromium.org/%d", currentRoll.Issue)

		if r.isMode(autoroll_modes.MODE_DRY_RUN) {
			if len(currentRoll.TryResults) > 0 && currentRoll.AllTrybotsFinished() {
				result := autoroll.ROLL_RESULT_DRY_RUN_FAILURE
				status := STATUS_DRY_RUN_FAILURE
				if currentRoll.AllTrybotsSucceeded() {
					result = autoroll.ROLL_RESULT_DRY_RUN_SUCCESS
					status = STATUS_DRY_RUN_SUCCESS
				}
				glog.Infof("Dry run is finished: %v", currentRoll)
				if currentRoll.RollingTo != r.rm.ChildHead() {
					if err := r.closeIssue(currentRoll, result, fmt.Sprintf("Repo has passed %s; will open a new dry run.", currentRoll.RollingTo)); err != nil {
						return STATUS_ERROR, err
					}
				} else if currentRoll.Result != result {
					// The dry run just finished. Set its result.
					if result == autoroll.ROLL_RESULT_DRY_RUN_FAILURE {
						if err := r.closeIssue(currentRoll, result, "Dry run failed. Closing, will open another."); err != nil {
							return STATUS_ERROR, err
						}
					} else {
						if err := r.addIssueComment(currentRoll, "Dry run finished successfully; leaving open in case we want to land"); err != nil {
							return STATUS_ERROR, err
						}
						currentRoll.Result = result
						if err := r.recent.Update(currentRoll); err != nil {
							return STATUS_ERROR, err
						}
						return status, nil
					}
				} else {
					// The dry run is finished but still good. Leave it open.
					glog.Infof("Dry run is finished and still good.")
					return status, nil
				}
			} else {
				if !currentRoll.CommitQueueDryRun {
					// Set it to dry-run only.
					glog.Infof("Setting dry-run bit on https://codereview.chromium.org/%d", currentRoll.Issue)
					if err := r.setDryRun(currentRoll, true); err != nil {
						return STATUS_ERROR, err
					}
				}
				glog.Infof("Dry run still in progress.")
				return STATUS_DRY_RUN_IN_PROGRESS, nil
			}
		} else {
			if currentRoll.CommitQueueDryRun {
				glog.Infof("Unsetting dry run bit on https://codereview.chromium.org/%d", currentRoll.Issue)
				if err := r.setDryRun(currentRoll, false); err != nil {
					return STATUS_ERROR, err
				}
			}
			if r.isMode(autoroll_modes.MODE_STOPPED) {
				// If we're stopped, close the issue.
				// Respect the previous result of the roll.
				if err := r.closeIssue(currentRoll, r.makeRollResult(currentRoll), "AutoRoller is stopped; closing the active roll."); err != nil {
					return STATUS_ERROR, err
				}
			} else if !currentRoll.CommitQueue {
				// If the CQ failed, close the issue.
				// Special case: if the current roll was a dry run which succeeded, land it.
				if currentRoll.Result == autoroll.ROLL_RESULT_DRY_RUN_SUCCESS {
					glog.Infof("Dry run succeeded. Attempting to land.")
					if err := r.setDryRun(currentRoll, false); err != nil {
						return STATUS_ERROR, nil
					}
					return STATUS_IN_PROGRESS, nil
				} else {
					if err := r.closeIssue(currentRoll, autoroll.ROLL_RESULT_FAILURE, "Commit queue failed; closing this roll."); err != nil {
						return STATUS_ERROR, err
					}
				}
			} else if time.Since(currentRoll.Modified) > 24*time.Hour {
				// If the roll has been open too long, close the issue.
				if err := r.closeIssue(currentRoll, autoroll.ROLL_RESULT_FAILURE, "Roll has been open for over 24 hours; closing."); err != nil {
					return STATUS_ERROR, err
				}
			} else if r.rm.RolledPast(currentRoll.RollingTo) {
				// If we've already rolled past the target revision, close the issue
				if err := r.closeIssue(currentRoll, autoroll.ROLL_RESULT_FAILURE, fmt.Sprintf("Already rolled past %s; closing this roll.", currentRoll.RollingTo)); err != nil {
					return STATUS_ERROR, err
				}
			} else {
				// Current roll is still good.
				glog.Infof("Roll is still active (%d): %s", currentRoll.Issue, currentRoll.Subject)
				return STATUS_IN_PROGRESS, nil
			}
		}
	}

	// If we're stopped, exit.
	if r.isMode(autoroll_modes.MODE_STOPPED) {
		glog.Infof("Roller is stopped; not opening new rolls.")
		return STATUS_STOPPED, nil
	}

	// If we're up-to-date, exit.
	if r.rm.LastRollRev() == r.rm.ChildHead() {
		glog.Infof("Repo is up-to-date.")
		return STATUS_UP_TO_DATE, nil
	}

	// Create a new roll.
	if r.attemptCounter.Get() >= ROLL_ATTEMPT_THROTTLE_NUM {
		return STATUS_THROTTLED, nil
	}
	r.attemptCounter.Inc()
	uploadedNum, err := r.rm.CreateNewRoll(r.GetEmails(), r.cqExtraTrybots, r.isMode(autoroll_modes.MODE_DRY_RUN))
	if err != nil {
		return STATUS_ERROR, fmt.Errorf("Failed to upload a new roll: %s", err)
	}
	glog.Infof("Uploaded new DEPS roll: %s/%d", autoroll.RIETVELD_URL, uploadedNum)
	uploaded, err := r.retrieveRoll(uploadedNum)
	if err != nil {
		return STATUS_ERROR, fmt.Errorf("Failed to retrieve uploaded roll: %s", err)
	}
	if err := r.recent.Add(uploaded); err != nil {
		return STATUS_ERROR, fmt.Errorf("Failed to insert uploaded roll into database: %s", err)
	}

	if r.isMode(autoroll_modes.MODE_DRY_RUN) {
		return STATUS_DRY_RUN_IN_PROGRESS, nil
	}
	return STATUS_IN_PROGRESS, nil
}

func (r *AutoRoller) User() string {
	return r.rm.User()
}
