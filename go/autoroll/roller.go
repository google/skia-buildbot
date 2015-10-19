package autoroll

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/util"
)

const (
	REPO_CHROMIUM = "https://chromium.googlesource.com/chromium/src.git"
	REPO_SKIA     = "https://skia.googlesource.com/skia.git"

	MODE_RUNNING = "running"
	MODE_STOPPED = "stopped"
	MODE_DRY_RUN = "dry run"

	STATUS_IDLE        = "idle"
	STATUS_IN_PROGRESS = "in progress"
	STATUS_STOPPED     = "stopped"

	TMPL_CQ_INCLUDE_TRYBOTS = "CQ_INCLUDE_TRYBOTS=%s"
)

var (
	VALID_MODES = []string{
		MODE_RUNNING,
		MODE_STOPPED,
		MODE_DRY_RUN,
	}

	VALID_STATUSES = []string{
		STATUS_IDLE,
		STATUS_IN_PROGRESS,
		STATUS_STOPPED,
	}
)

// Mode indicates the user-controlled running mode of the AutoRoll Bot.
type Mode string

// Status indicates the last activity of the bot.
type Status string

// AutoRoller is a struct used for managing DEPS rolls.
type AutoRoller struct {
	chromiumDir       string
	chromiumParentDir string
	cqExtraTrybots    []string
	currentRoll       *AutoRollIssue
	emails            []string
	includeCommitLog  bool
	mode              Mode
	mtx               sync.RWMutex
	recent            []*AutoRollIssue
	rietveld          *rietveld.Rietveld
	skiaDir           string
	status            Status
	workdir           string
}

// NewAutoRoller creates and returns a new AutoRoller which runs at the given frequency.
func NewAutoRoller(workdir string, cqExtraTrybots, emails []string, rietveld *rietveld.Rietveld, frequency time.Duration) (*AutoRoller, error) {
	chromiumParentDir := path.Join(workdir, "chromium")
	// TODO(borenet): Do this in a smarter way; rather than contually loading
	// the last N rolls, update the "recent" list as we upload/close CLs.
	recent, err := GetLastNRolls(POLLER_ROLLS_LIMIT)
	if err != nil {
		return nil, err
	}
	arb := &AutoRoller{
		chromiumDir:       path.Join(chromiumParentDir, "src"),
		chromiumParentDir: chromiumParentDir,
		cqExtraTrybots:    cqExtraTrybots,
		emails:            emails,
		includeCommitLog:  true,
		mode:              MODE_RUNNING,
		recent:            recent,
		rietveld:          rietveld,
		skiaDir:           path.Join(workdir, "skia"),
		status:            STATUS_IDLE,
		workdir:           workdir,
	}

	go func() {
		util.LogErr(arb.doAutoRoll())
		for _ = range time.Tick(frequency) {
			util.LogErr(arb.doAutoRoll())
		}
	}()

	return arb, nil
}

// SetMode sets the desired mode of the bot. This has no effect on the
// behavior of the bot until its next cycle.
func (r *AutoRoller) SetMode(m Mode) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if !util.In(string(m), VALID_MODES) {
		return fmt.Errorf("Invalid mode: %s", m)
	}
	r.mode = m
	return nil
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
func (r *AutoRoller) setStatus(s Status, currentRoll *AutoRollIssue) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if !util.In(string(s), VALID_STATUSES) {
		return fmt.Errorf("Invalid status: %s", s)
	}
	if s == STATUS_IDLE {
		if currentRoll != nil {
			return fmt.Errorf("Cannot be in idle status with an active roll.")
		}
	} else if s == STATUS_STOPPED {
		if currentRoll != nil {
			return fmt.Errorf("Cannot be in stopped status with an active roll.")
		}
	} else if s == STATUS_IN_PROGRESS {
		if currentRoll == nil {
			return fmt.Errorf("Cannot be in in-progress status with no active roll.")
		}
	}
	r.status = s
	r.currentRoll = currentRoll
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

// GetCQExtraTrybots returns the list of trybots which are added to the commit
// queue in addition to the default set.
func (r *AutoRoller) GetCQExtraTrybots() []string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.cqExtraTrybots
}

// SetCQExtraTrybots sets the list of trybots which are added to the commit
// queue in addition to the default set.
func (r *AutoRoller) SetCQExtraTrybots(c []string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.cqExtraTrybots = c
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

// GetRecentRolls retrieves the list of recent DEPS rolls.
func (r *AutoRoller) GetRecentRolls() []*AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.recent
}

// setRecentRolls sets the list of recent DEPS rolls.
func (r *AutoRoller) setRecentRolls(recent []*AutoRollIssue) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.recent = recent
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
func (r *AutoRoller) getCurrentRollRev(subject string, skiaRepo *gitinfo.GitInfo) (string, error) {
	matches := ROLL_REV_REGEX.FindStringSubmatch(subject)
	if matches == nil {
		return "", fmt.Errorf("No roll revision found in %q", subject)
	}
	return skiaRepo.FullHash(matches[1])
}

// getLastRollRev returns the commit hash of the last-completed DEPS roll.
func (r *AutoRoller) getLastRollRev() (string, error) {
	output, err := exec.RunCwd(r.chromiumDir, "gclient", "revinfo")
	if err != nil {
		return "", err
	}
	split := strings.Split(output, "\n")
	for _, s := range split {
		if strings.HasPrefix(s, "src/third_party/skia") {
			subs := strings.Split(s, "@")
			if len(subs) != 2 {
				return "", fmt.Errorf("Failed to parse output of `gclient revinfo`")
			}
			return subs[1], nil
		}
	}
	return "", fmt.Errorf("Failed to parse output of `gclient revinfo`")
}

// shouldCloseCurrentRoll determines whether the currently-active DEPS roll
// should be closed in favor of a new one. Returns a boolean indicating whether
// the current roll should be closed, a message to be used with the roll
// closure, if applicable, or an error.
func (r *AutoRoller) shouldCloseCurrentRoll(currentRoll *AutoRollIssue, skiaRepo *gitinfo.GitInfo, lastRollRev string) (bool, string, error) {
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
	rollingTo, err := r.getCurrentRollRev(currentRoll.Subject, skiaRepo)
	if err != nil {
		return false, "", err
	}
	lastDetails, err := skiaRepo.Details(lastRollRev)
	if err != nil {
		return false, "", err
	}
	rollingToDetails, err := skiaRepo.Details(rollingTo)
	if err != nil {
		return false, "", err
	}
	if lastDetails.Timestamp.After(rollingToDetails.Timestamp) {
		return true, fmt.Sprintf("Already rolled past %s; closing this roll.", rollingTo), nil
	}

	// Roll is still good; don't close the issue.
	return false, "", nil
}

// createNewRoll creates and uploads a new DEPS roll from the given commit to
// a new commit. It returns the uploaded issue or any error.
func (r *AutoRoller) createNewRoll(from, to string) (*AutoRollIssue, error) {
	// Clean the checkout, get onto a fresh branch.
	rollBranch := "skia_roll"
	if _, err := exec.RunCwd(r.chromiumDir, "git", "clean", "-d", "-f"); err != nil {
		return nil, err
	}
	_, _ = exec.RunCwd(r.chromiumDir, "git", "rebase", "--abort")
	_, _ = exec.RunCwd(r.chromiumDir, "git", "branch", "-D", rollBranch)
	if _, err := exec.RunCwd(r.chromiumDir, "git", "checkout", "origin/master", "-f"); err != nil {
		return nil, err
	}
	if _, err := exec.RunCwd(r.chromiumDir, "git", "checkout", "-b", rollBranch, "-t", "origin/master", "-f"); err != nil {
		return nil, err
	}

	// Defer some more cleanup.
	defer func() {
		_, _ = exec.RunCwd(r.chromiumDir, "git", "checkout", "origin/master", "-f")
		_, _ = exec.RunCwd(r.chromiumDir, "git", "branch", "-D", rollBranch)
	}()

	// Create the roll CL.
	if _, err := exec.RunCwd(r.chromiumDir, "roll-dep", "src/third_party/skia", to); err != nil {
		return nil, err
	}
	// Build the commit message, starting with the message provided by roll-dep.
	commitMsg, err := exec.RunCwd(r.chromiumDir, "git", "log", "-n1", "--format=%B", "HEAD")
	if err != nil {
		return nil, err
	}
	cqExtraTrybots := r.GetCQExtraTrybots()
	if cqExtraTrybots != nil && len(cqExtraTrybots) > 0 {
		commitMsg += "\n" + fmt.Sprintf(TMPL_CQ_INCLUDE_TRYBOTS, strings.Join(cqExtraTrybots, ","))
	}
	uploadCmd := []string{"git", "cl", "upload", "--bypass-hooks", "-f"}
	if r.isMode(MODE_DRY_RUN) {
		uploadCmd = append(uploadCmd, "--cq-dry-run")
	} else {
		uploadCmd = append(uploadCmd, "--use-commit-queue")
	}
	tbr := "\nTBR="
	emails := r.GetEmails()
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		tbr += emailStr
		uploadCmd = append(uploadCmd, "--send-mail", "--cc", emailStr)
	}
	commitMsg += tbr
	uploadCmd = append(uploadCmd, "-m", commitMsg)

	// Upload the CL.
	if _, err := exec.RunCwd(r.chromiumDir, uploadCmd...); err != nil {
		return nil, err
	}

	issues, err := r.findActiveRolls()
	if err != nil {
		return nil, err
	}
	if len(issues) != 1 {
		return nil, fmt.Errorf("Found too many open rolls during upload.")
	}
	return issues[0], nil
}

// doAutoRoll is the primary method of the AutoRoll Bot. It runs on a timer,
// updates checkouts, manages active roll CLs, and uploads new rolls. It sets
// the status of the bot which may be read by users.
func (r *AutoRoller) doAutoRoll() error {
	err1 := r.doAutoRollInner()
	// TODO(borenet): Do this in a smarter way; rather than contually loading
	// the last N rolls, update the "recent" list as we upload/close CLs.
	recent, err2 := GetLastNRolls(POLLER_ROLLS_LIMIT)
	if err2 == nil {
		r.recent = recent
	}
	if err1 != nil {
		return err1
	}
	return err2
}

// doAutoRollInner is the main workhorse for doAutoRoll.
func (r *AutoRoller) doAutoRollInner() error {
	// Sync the projects.
	skiaRepo, err := gitinfo.CloneOrUpdate(REPO_SKIA, r.skiaDir, true)
	if err != nil {
		return err
	}
	if _, err := exec.RunCwd(r.chromiumParentDir, "gclient", "config", REPO_CHROMIUM); err != nil {
		return err
	}
	if _, err := exec.RunCwd(r.chromiumParentDir, "gclient", "sync", "--nohooks"); err != nil {
		return err
	}

	// Get the last roll revision.
	lastRollRev, err := r.getLastRollRev()
	if err != nil {
		return err
	}

	// Find the active roll, if it exists.
	activeRolls, err := r.findActiveRolls()
	if err != nil {
		return err
	}
	// Close any extra rolls.
	var currentRoll *AutoRollIssue
	for len(activeRolls) > 1 {
		if err := r.rietveld.Close(activeRolls[0].Issue, "Multiple DEPS rolls found; closing all but the newest."); err != nil {
			return err
		}
		activeRolls = activeRolls[1:]
	}
	if len(activeRolls) == 1 {
		currentRoll = activeRolls[0]
	}

	// There's a currently-active roll. Determine whether or not it's still good.
	// If so, leave it open and exit. If not, close it so that we can open another.
	if currentRoll != nil {
		shouldClose, msg, err := r.shouldCloseCurrentRoll(currentRoll, skiaRepo, lastRollRev)
		if err != nil {
			return err
		}
		if shouldClose {
			if err := r.closeIssue(currentRoll, msg); err != nil {
				return err
			}
		} else {
			// Current roll is still good. Exit.
			if err := r.setStatus(STATUS_IN_PROGRESS, currentRoll); err != nil {
				return err
			}
			glog.Infof("Roll is still active (%d): %s", currentRoll.Issue, currentRoll.Subject)
			return nil
		}
	}
	if err := r.setStatus(STATUS_IDLE, nil); err != nil {
		return err
	}

	// If we're stopped, exit.
	if r.isMode(MODE_STOPPED) {
		if err := r.setStatus(STATUS_STOPPED, nil); err != nil {
			return err
		}
		glog.Infof("Roller is stopped; not opening new rolls.")
		return nil
	}

	// If we're up-to-date, exit.
	skiaHead, err := skiaRepo.FullHash("origin/master")
	if err != nil {
		return err
	}
	if lastRollRev == skiaHead {
		glog.Infof("Skia is up-to-date.")
		return nil
	}

	// Create a new roll.
	newRoll, err := r.createNewRoll(lastRollRev, skiaHead)
	if err != nil {
		return err
	}
	if err := r.setStatus(STATUS_IN_PROGRESS, newRoll); err != nil {
		return err
	}
	return nil
}
