package autoroll

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/util"
)

const (
	REPO_CHROMIUM = "https://chromium.googlesource.com/chromium/src.git"
	REPO_SKIA     = "https://skia.googlesource.com/skia.git"

	MODE_RUNNING = "running"
	MODE_STOPPED = "stopped"
	MODE_DRY_RUN = "dry run"

	ROLL_RESULT_IN_PROGRESS = "in progress"
	ROLL_RESULT_SUCCESS     = "succeeded"
	ROLL_RESULT_FAILURE     = "failed"

	STATUS_ERROR       = "error"
	STATUS_IN_PROGRESS = "in progress"
	STATUS_STOPPED     = "stopped"
	STATUS_UP_TO_DATE  = "up to date"

	TMPL_CQ_INCLUDE_TRYBOTS = "CQ_INCLUDE_TRYBOTS=%s"
)

var (
	VALID_MODES = []string{
		MODE_RUNNING,
		MODE_STOPPED,
		MODE_DRY_RUN,
	}

	VALID_STATUSES = []string{
		STATUS_ERROR,
		STATUS_IN_PROGRESS,
		STATUS_STOPPED,
		STATUS_UP_TO_DATE,
	}
)

// Mode indicates the user-controlled running mode of the AutoRoll Bot.
type Mode string

// Status indicates the last activity of the bot.
type Status string

// AutoRoller is a struct used for managing DEPS rolls.
type AutoRoller struct {
	cqExtraTrybots   []string
	currentRoll      *AutoRollIssue
	emails           []string
	includeCommitLog bool
	lastError        error
	lastRoll         *AutoRollIssue
	mode             Mode
	mtx              sync.RWMutex
	recent           []*AutoRollIssue
	rm               *repoManager
	rietveld         *rietveld.Rietveld
	runningMtx       sync.Mutex
	status           Status
}

// NewAutoRoller creates and returns a new AutoRoller which runs at the given frequency.
func NewAutoRoller(workdir string, cqExtraTrybots, emails []string, rietveld *rietveld.Rietveld, tickFrequency, repoFrequency time.Duration) (*AutoRoller, error) {
	rm, err := newRepoManager(workdir, cqExtraTrybots, emails, repoFrequency)
	if err != nil {
		return nil, err
	}
	arb := &AutoRoller{
		includeCommitLog: true,
		mode:             MODE_RUNNING,
		rm:               rm,
		rietveld:         rietveld,
		status:           STATUS_ERROR,
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

// SetMode sets the desired mode of the bot. This forces the bot to run and
// blocks until it finishes.
func (r *AutoRoller) SetMode(m Mode) error {
	if !util.In(string(m), VALID_MODES) {
		return fmt.Errorf("Invalid mode: %s", m)
	}
	r.mtx.Lock()
	r.mode = m
	r.mtx.Unlock()
	return r.doAutoRoll()
}

// GetMode returns the user-controlled desired mode of the bot.
func (r *AutoRoller) GetMode() Mode {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.mode
}

// isMode determines whether the bot is in the given mode.
func (r *AutoRoller) isMode(s Mode) bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.mode == s
}

// setStatus sets the current reporting status of the bot.
func (r *AutoRoller) setStatus(s Status, lastError error) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if !util.In(string(s), VALID_STATUSES) {
		return fmt.Errorf("Invalid status: %s", s)
	}
	if s == STATUS_ERROR {
		if lastError == nil {
			return fmt.Errorf("Cannot set error status without an error!")
		}
	} else if lastError != nil {
		return fmt.Errorf("Cannot be in any status other than error when an error occurred.")
	}
	r.status = s
	r.lastError = lastError
	return nil
}

// GetStatus returns the current reporting status of the bot.
func (r *AutoRoller) GetStatus() Status {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.status
}

// GetCurrentRoll returns the currently active DEPS roll, if one exists, and
// nil if no such roll exists.
func (r *AutoRoller) GetCurrentRoll() *AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.currentRoll
}

// GetLastRoll returns the last-completed DEPS roll.
func (r *AutoRoller) GetLastRoll() *AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.lastRoll
}

// GetError returns the error encountered on the last cycle of the roller, if
// applicable.
func (r *AutoRoller) GetError() error {
	return r.lastError
}

// GetCQExtraTrybots returns the list of trybots which are added to the commit
// queue in addition to the default set.
func (r *AutoRoller) GetCQExtraTrybots() []string {
	return r.rm.GetCQExtraTrybots()
}

// SetCQExtraTrybots sets the list of trybots which are added to the commit
// queue in addition to the default set.
func (r *AutoRoller) SetCQExtraTrybots(c []string) {
	r.rm.SetCQExtraTrybots(c)
}

// GetEmails returns the list of email addresses which are copied on DEPS rolls.
func (r *AutoRoller) GetEmails() []string {
	return r.rm.GetEmails()
}

// SetEmails sets the list of email addresses which are copied on DEPS rolls.
func (r *AutoRoller) SetEmails(e []string) {
	r.rm.SetEmails(e)
}

// GetRecentRolls retrieves the list of recent DEPS rolls.
func (r *AutoRoller) GetRecentRolls() []*AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.recent
}

// updateRecentRolls sets the list of recent DEPS rolls.
func (r *AutoRoller) updateRecentRolls() error {
	// Load the last N rolls.
	recent, err := GetLastNRolls(POLLER_ROLLS_LIMIT)
	if err != nil {
		return err
	}

	// Find the current and last rolls.
	var currentRoll *AutoRollIssue
	var lastRoll *AutoRollIssue
	for _, roll := range recent {
		// Any open issue is the current roll.
		if currentRoll == nil && !roll.Closed {
			currentRoll = roll
		}
		// The first closed issue is the last roll.
		if lastRoll == nil && roll.Closed {
			lastRoll = roll
		}
		// Shortcut.
		if currentRoll != nil && lastRoll != nil {
			break
		}
	}

	// Retrieve try results for the current and last rolls.
	if currentRoll != nil {
		tries, err := r.getTryResults(currentRoll)
		if err != nil {
			return err
		}
		currentRoll.TryResults = tries
	}
	if lastRoll != nil {
		tries, err := r.getTryResults(lastRoll)
		if err != nil {
			return err
		}
		lastRoll.TryResults = tries
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.recent = recent
	r.currentRoll = currentRoll
	r.lastRoll = lastRoll
	return nil
}

// getTryResults returns trybot results for the given roll.
func (r *AutoRoller) getTryResults(roll *AutoRollIssue) ([]*TryResult, error) {
	tries, err := r.rietveld.GetTrybotResults(roll.Issue, roll.Patchsets[len(roll.Patchsets)-1])
	if err != nil {
		return nil, err
	}
	res := make([]*TryResult, 0, len(tries))
	for _, t := range tries {
		var params struct {
			Builder string `json:"builder_name"`
		}
		if err := json.Unmarshal([]byte(t.ParametersJson), &params); err != nil {
			return nil, err
		}
		res = append(res, &TryResult{
			Builder: params.Builder,
			Result:  t.Result,
			Status:  t.Status,
			Url:     t.Url,
		})
	}
	sort.Sort(tryResultSlice(res))
	return res, nil
}

// closeIssue closes the given issue with the given message.
func (r *AutoRoller) closeIssue(issue *AutoRollIssue, msg string) error {
	glog.Infof("Closing issue %d with message: %s", issue.Issue, msg)
	return r.rietveld.Close(issue.Issue, msg)
}

// findActiveRolls retrieves a slice of Issues which fit the criteria to be
// considered DEPS rolls.
func (r *AutoRoller) findActiveRolls() ([]*AutoRollIssue, error) {
	return search(r.rietveld, 100, rietveld.SearchOpen(true))
}

// getCurrentRollRev parses an abbreviated commit hash from the given issue
// subject and returns the full hash.
func (r *AutoRoller) getCurrentRollRev(subject string) (string, error) {
	matches := ROLL_REV_REGEX.FindStringSubmatch(subject)
	if matches == nil {
		return "", fmt.Errorf("No roll revision found in %q", subject)
	}
	return r.rm.FullSkiaHash(matches[1])
}

// shouldCloseCurrentRoll determines whether the currently-active DEPS roll
// should be closed in favor of a new one. Returns a boolean indicating whether
// the current roll should be closed, a message to be used with the roll
// closure, if applicable, or an error.
func (r *AutoRoller) shouldCloseCurrentRoll(currentRoll *AutoRollIssue) (bool, string, error) {
	glog.Infof("Found current roll: https://codereview.chromium.org/%d", currentRoll.Issue)
	// If we're stopped, close the issue.
	if r.isMode(MODE_STOPPED) {
		return true, "AutoRoller is stopped; closing the active roll.", nil
	}

	// TODO(borenet): If we're in dry run mode, don't kill the CL.

	// If the CQ failed, close the issue.
	if !currentRoll.CommitQueue {
		return true, "Commit queue failed; closing this roll.", nil
	}

	// If the roll has been open too long, close the issue.
	if time.Since(currentRoll.Modified) > 24*time.Hour {
		return true, "Roll has been open for over 24 hours; closing.", nil
	}

	// If we've already rolled past the target revision, close the issue.
	rollingTo, err := r.getCurrentRollRev(currentRoll.Subject)
	if err != nil {
		return false, "", err
	}
	if r.rm.RolledPast(rollingTo) {
		return true, fmt.Sprintf("Already rolled past %s; closing this roll.", rollingTo), nil
	}

	// Roll is still good; don't close the issue.
	return false, "", nil
}

// doAutoRoll is the primary method of the AutoRoll Bot. It runs on a timer,
// updates checkouts, manages active roll CLs, and uploads new rolls. It sets
// the status of the bot which may be read by users.
func (r *AutoRoller) doAutoRoll() error {
	status, lastError := r.doAutoRollInner()

	if err := r.updateRecentRolls(); err != nil {
		return err
	}

	if err := r.setStatus(status, lastError); err != nil {
		return err
	}

	return lastError
}

// doAutoRollInner does the actual work of the AutoRoll.
func (r *AutoRoller) doAutoRollInner() (Status, error) {
	r.runningMtx.Lock()
	defer r.runningMtx.Unlock()

	// Find the active roll, if it exists.
	activeRolls, err := r.findActiveRolls()
	if err != nil {
		return STATUS_ERROR, err
	}
	// Close any extra rolls.
	var currentRoll *AutoRollIssue
	for len(activeRolls) > 1 {
		if err := r.rietveld.Close(activeRolls[0].Issue, "Multiple DEPS rolls found; closing all but the newest."); err != nil {
			return STATUS_ERROR, err
		}
		activeRolls = activeRolls[1:]
	}
	if len(activeRolls) == 1 {
		currentRoll = activeRolls[0]
	}

	// There's a currently-active roll. Determine whether or not it's still good.
	// If so, leave it open and exit. If not, close it so that we can open another.
	if currentRoll != nil {
		shouldClose, msg, err := r.shouldCloseCurrentRoll(currentRoll)
		if err != nil {
			return STATUS_ERROR, err
		}
		if shouldClose {
			if err := r.closeIssue(currentRoll, msg); err != nil {
				return STATUS_ERROR, err
			}
		} else {
			// Current roll is still good. Exit.
			glog.Infof("Roll is still active (%d): %s", currentRoll.Issue, currentRoll.Subject)
			return STATUS_IN_PROGRESS, nil
		}
	}

	// If we're stopped, exit.
	if r.isMode(MODE_STOPPED) {
		glog.Infof("Roller is stopped; not opening new rolls.")
		return STATUS_STOPPED, nil
	}

	// If we're up-to-date, exit.
	if r.rm.LastRollRev() == r.rm.SkiaHead() {
		glog.Infof("Skia is up-to-date.")
		return STATUS_UP_TO_DATE, nil
	}

	// Create a new roll.
	if err := r.rm.CreateNewRoll(r.isMode(MODE_DRY_RUN)); err != nil {
		return STATUS_ERROR, err
	}

	return STATUS_IN_PROGRESS, nil
}
