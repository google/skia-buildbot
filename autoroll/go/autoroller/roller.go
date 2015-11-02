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
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/util"
)

const (
	STATUS_ERROR       = "error"
	STATUS_IN_PROGRESS = "in progress"
	STATUS_STOPPED     = "stopped"
	STATUS_UP_TO_DATE  = "up to date"
)

var (
	VALID_STATUSES = []string{
		STATUS_ERROR,
		STATUS_IN_PROGRESS,
		STATUS_STOPPED,
		STATUS_UP_TO_DATE,
	}
)

// AutoRoller is a struct used for managing DEPS rolls.
type AutoRoller struct {
	cqExtraTrybots   []string
	emails           []string
	includeCommitLog bool
	lastError        error
	modeHistory      *autoroll_modes.ModeHistory
	mtx              sync.RWMutex
	recent           *recent_rolls.RecentRolls
	rm               repo_manager.RepoManager
	rietveld         *rietveld.Rietveld
	runningMtx       sync.Mutex
	status           string
}

// NewAutoRoller creates and returns a new AutoRoller which runs at the given frequency.
func NewAutoRoller(workdir string, cqExtraTrybots, emails []string, rietveld *rietveld.Rietveld, tickFrequency, repoFrequency time.Duration) (*AutoRoller, error) {
	rm, err := repo_manager.NewRepoManager(workdir, repoFrequency)
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
		cqExtraTrybots:   cqExtraTrybots,
		emails:           emails,
		includeCommitLog: true,
		modeHistory:      mh,
		recent:           recent,
		rietveld:         rietveld,
		rm:               rm,
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
	Error       error                     `json:"error"`
	LastRoll    *autoroll.AutoRollIssue   `json:"lastRoll"`
	Mode        string                    `json:"mode"`
	Recent      []*autoroll.AutoRollIssue `json:"recent"`
	Status      string                    `json:"status"`
	ValidModes  []string                  `json:"validModes"`
}

// GetStatus returns the roll-up status of the bot.
func (r *AutoRoller) GetStatus(includeError bool) AutoRollStatus {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	recent := r.recent.GetRecentRolls()

	current := r.recent.CurrentRoll()
	if current != nil {
		current = &(*current)
	}

	last := r.recent.LastRoll()
	if last != nil {
		last = &(*last)
	}

	s := AutoRollStatus{
		CurrentRoll: current,
		LastRoll:    last,
		Mode:        r.modeHistory.CurrentMode(),
		Recent:      recent,
		Status:      r.status,
		ValidModes:  autoroll_modes.VALID_MODES,
	}
	if includeError {
		s.Error = r.lastError
	}
	return s
}

// SetMode sets the desired mode of the bot. This forces the bot to run and
// blocks until it finishes.
func (r *AutoRoller) SetMode(m, user, message string) error {
	if err := r.modeHistory.Add(m, user, message); err != nil {
		return err
	}
	return r.doAutoRoll()
}

// isMode determines whether the bot is in the given mode.
func (r *AutoRoller) isMode(s string) bool {
	return r.modeHistory.CurrentMode() == s
}

// setStatus sets the current reporting status of the bot.
func (r *AutoRoller) setStatus(s string, lastError error) error {
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

// GetEmails returns the list of email addresses which are copied on DEPS rolls.
func (r *AutoRoller) GetEmails() []string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.emails
}

// SetEmails sets the list of email addresses which are copied on DEPS rolls.
func (r *AutoRoller) SetEmails(e []string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.emails = e
}

// closeIssue closes the given issue with the given message.
func (r *AutoRoller) closeIssue(issue *autoroll.AutoRollIssue, result, msg string) error {
	glog.Infof("Closing issue %d with message: %s", issue.Issue, msg)
	if err := r.rietveld.Close(issue.Issue, msg); err != nil {
		return err
	}
	issue.Result = result
	issue.Closed = true
	return r.recent.Update(issue)
}

// getRollRev parses an abbreviated commit hash from the given issue
// subject and returns the full hash.
func (r *AutoRoller) getRollRev(subject string) (string, error) {
	matches := autoroll.ROLL_REV_REGEX.FindStringSubmatch(subject)
	if matches == nil {
		return "", fmt.Errorf("No roll revision found in %q", subject)
	}
	return r.rm.FullSkiaHash(matches[1])
}

// updateCurrentRoll retrieves updated information about the current DEPS roll.
func (r *AutoRoller) updateCurrentRoll() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	currentRoll := r.recent.CurrentRoll()
	if currentRoll == nil {
		return nil
	}

	updated, err := r.retrieveRoll(currentRoll.Issue)
	if err != nil {
		return err
	}

	return r.recent.Update(updated)
}

// retrieveRoll obtains the given DEPS roll from Rietveld.
func (r *AutoRoller) retrieveRoll(issueNum int64) (*autoroll.AutoRollIssue, error) {
	issue, err := r.rietveld.GetIssueProperties(issueNum, true)
	if err != nil {
		return nil, err
	}
	a := autoroll.FromRietveldIssue(issue)
	tryResults, err := autoroll.GetTryResults(a)
	if err != nil {
		return nil, err
	}
	a.TryResults = tryResults
	return a, nil
}

// doAutoRoll is the primary method of the AutoRoll Bot. It runs on a timer,
// updates checkouts, manages active roll CLs, and uploads new rolls. It sets
// the status of the bot which may be read by users.
func (r *AutoRoller) doAutoRoll() error {
	status, lastError := r.doAutoRollInner()

	if err := r.setStatus(status, lastError); err != nil {
		return err
	}

	return lastError
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

		rollingTo, err := r.getRollRev(currentRoll.Subject)
		if err != nil {
			return STATUS_ERROR, err
		}

		if r.isMode(autoroll_modes.MODE_STOPPED) {
			// If we're stopped, close the issue.
			if err := r.closeIssue(currentRoll, autoroll.ROLL_RESULT_FAILURE, "AutoRoller is stopped; closing the active roll."); err != nil {
				return STATUS_ERROR, err
			}
		} else if !currentRoll.CommitQueue {
			// If the CQ failed, close the issue.
			if err := r.closeIssue(currentRoll, autoroll.ROLL_RESULT_FAILURE, "Commit queue failed; closing this roll."); err != nil {
				return STATUS_ERROR, err
			}
		} else if time.Since(currentRoll.Modified) > 24*time.Hour {
			// If the roll has been open too long, close the issue.
			if err := r.closeIssue(currentRoll, autoroll.ROLL_RESULT_FAILURE, "Roll has been open for over 24 hours; closing."); err != nil {
				return STATUS_ERROR, err
			}
		} else if r.rm.RolledPast(rollingTo) {
			// If we've already rolled past the target revision, close the issue
			if err := r.closeIssue(currentRoll, autoroll.ROLL_RESULT_FAILURE, fmt.Sprintf("Already rolled past %s; closing this roll.", rollingTo)); err != nil {
				return STATUS_ERROR, err
			}
		} else {
			// Current roll is still good. Exit.
			glog.Infof("Roll is still active (%d): %s", currentRoll.Issue, currentRoll.Subject)
			return STATUS_IN_PROGRESS, nil
		}
	}

	// If we're stopped, exit.
	if r.isMode(autoroll_modes.MODE_STOPPED) {
		glog.Infof("Roller is stopped; not opening new rolls.")
		return STATUS_STOPPED, nil
	}

	// If we're up-to-date, exit.
	if r.rm.LastRollRev() == r.rm.SkiaHead() {
		glog.Infof("Skia is up-to-date.")
		return STATUS_UP_TO_DATE, nil
	}

	// Create a new roll.
	uploadedNum, err := r.rm.CreateNewRoll(r.GetEmails(), r.cqExtraTrybots, r.isMode(autoroll_modes.MODE_DRY_RUN))
	if err != nil {
		return STATUS_ERROR, err
	}
	uploaded, err := r.retrieveRoll(uploadedNum)
	if err != nil {
		return STATUS_ERROR, err
	}
	if err := r.recent.Add(uploaded); err != nil {
		return STATUS_ERROR, err
	}
	glog.Infof("Uploaded new DEPS roll: %s/%d", autoroll.RIETVELD_URL, uploadedNum)

	return STATUS_IN_PROGRESS, nil
}
