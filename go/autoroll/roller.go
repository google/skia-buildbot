package autoroll

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/util"
)

const (
	DB_FILENAME = "autoroll.db"

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
	db               *db
	emails           []string
	includeCommitLog bool
	lastError        error
	modeHistory      *modeHistory
	mtx              sync.RWMutex
	recent           *RecentRolls
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
	db, err := openDB(path.Join(workdir, DB_FILENAME))
	if err != nil {
		return nil, err
	}

	recent, err := newRecentRolls(db)
	if err != nil {
		return nil, err
	}

	mh, err := newModeHistory(db)
	if err != nil {
		return nil, err
	}

	arb := &AutoRoller{
		db:               db,
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

type AutoRollStatus struct {
	CurrentRoll *AutoRollIssue   `json:"currentRoll"`
	Error       error            `json:"error"`
	LastRoll    *AutoRollIssue   `json:"lastRoll"`
	Mode        string           `json:"mode"`
	Recent      []*AutoRollIssue `json:"recent"`
	Status      string           `json:"status"`
	ValidModes  []string         `json:"validModes"`
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

	s := AutoRollStatus{
		CurrentRoll: current,
		LastRoll:    r.recent.LastRoll(),
		Mode:        string(r.modeHistory.CurrentMode()),
		Recent:      recent,
		Status:      string(r.status),
		ValidModes:  VALID_MODES,
	}
	if includeError {
		s.Error = r.lastError
	}
	return s
}

// SetMode sets the desired mode of the bot. This forces the bot to run and
// blocks until it finishes.
func (r *AutoRoller) SetMode(m Mode, user string, message string) error {
	if !util.In(string(m), VALID_MODES) {
		return fmt.Errorf("Invalid mode: %s", m)
	}
	modeChange := &ModeChange{
		Message: message,
		Mode:    m,
		Time:    time.Now(),
		User:    user,
	}
	if err := r.modeHistory.Add(modeChange); err != nil {
		return err
	}
	return r.doAutoRoll()
}

// isMode determines whether the bot is in the given mode.
func (r *AutoRoller) isMode(s Mode) bool {
	return r.modeHistory.CurrentMode() == s
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

// SetEmails sets the list of email addresses which are copied on DEPS rolls.
func (r *AutoRoller) SetEmails(e []string) {
	r.rm.SetEmails(e)
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
func (r *AutoRoller) closeIssue(issue *AutoRollIssue, result, msg string) error {
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
	matches := ROLL_REV_REGEX.FindStringSubmatch(subject)
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
func (r *AutoRoller) retrieveRoll(issueNum int64) (*AutoRollIssue, error) {
	issue, err := r.rietveld.GetIssueProperties(issueNum, true)
	if err != nil {
		return nil, err
	}
	a := autoRollIssue(issue)
	tryResults, err := r.getTryResults(a)
	if err != nil {
		return nil, err
	}
	a.TryResults = tryResults
	a.Result = rollResult(a)
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
func (r *AutoRoller) doAutoRollInner() (Status, error) {
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

		if r.isMode(MODE_STOPPED) {
			// If we're stopped, close the issue.
			if err := r.closeIssue(currentRoll, ROLL_RESULT_FAILURE, "AutoRoller is stopped; closing the active roll."); err != nil {
				return STATUS_ERROR, err
			}
		} else if !currentRoll.CommitQueue {
			// If the CQ failed, close the issue.
			if err := r.closeIssue(currentRoll, ROLL_RESULT_FAILURE, "Commit queue failed; closing this roll."); err != nil {
				return STATUS_ERROR, err
			}
		} else if time.Since(currentRoll.Modified) > 24*time.Hour {
			// If the roll has been open too long, close the issue.
			if err := r.closeIssue(currentRoll, ROLL_RESULT_FAILURE, "Roll has been open for over 24 hours; closing."); err != nil {
				return STATUS_ERROR, err
			}
		} else if r.rm.RolledPast(rollingTo) {
			// If we've already rolled past the target revision, close the issue
			if err := r.closeIssue(currentRoll, ROLL_RESULT_FAILURE, fmt.Sprintf("Already rolled past %s; closing this roll.", rollingTo)); err != nil {
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
	uploadedNum, err := r.rm.CreateNewRoll(r.isMode(MODE_DRY_RUN))
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

	return STATUS_IN_PROGRESS, nil
}
