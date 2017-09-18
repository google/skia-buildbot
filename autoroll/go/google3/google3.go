/*
	Keep track of Skia rolls into Google3.

	Rolls are added/updated by POST/PUT request with webhook authentication.
*/

package google3

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
)

// AutoRoller provides a handler for adding/updating Rolls, translating them into AutoRollIssue for
// storage in RecentRolls. It also manages an AutoRollStatusCache for status handlers.
type AutoRoller struct {
	recent      *recent_rolls.RecentRolls
	status      *roller.AutoRollStatusCache
	childRepo   *git.Repo
	childBranch string
	liveness    metrics2.Liveness
}

func NewAutoRoller(workdir string, childRepoUrl string, childBranch string) (*AutoRoller, error) {
	recent, err := recent_rolls.NewRecentRolls(path.Join(workdir, "recent_rolls.bdb"))
	if err != nil {
		return nil, err
	}

	childRepo, err := git.NewRepo(childRepoUrl, workdir)
	if err != nil {
		util.LogErr(recent.Close())
		return nil, err
	}

	a := &AutoRoller{
		recent:      recent,
		status:      &roller.AutoRollStatusCache{},
		childRepo:   childRepo,
		childBranch: childBranch,
		liveness:    metrics2.NewLiveness("last_autoroll_landed"),
	}

	if err := a.childRepo.Update(); err != nil {
		util.LogErr(recent.Close())
		return nil, err
	}

	if err := a.UpdateStatus("", true); err != nil {
		util.LogErr(recent.Close())
		return nil, err
	}
	return a, nil
}

// Start ensures DBs are closed when ctx is canceled.
func (a *AutoRoller) Start(tickFrequency, repoFrequency time.Duration, ctx context.Context) {
	go func() {
		<-ctx.Done()
		util.LogErr(a.recent.Close())
	}()
	go util.RepeatCtx(repoFrequency, ctx, func() {
		if err := a.childRepo.Update(); err != nil {
			sklog.Error(err)
			return
		}
		util.LogErr(a.UpdateStatus("", true))
	})
}

func (a *AutoRoller) AddHandlers(r *mux.Router) {
	r.HandleFunc("/json/roll", a.rollHandler).Methods(http.MethodPost, http.MethodPut)
}

func (a *AutoRoller) GetStatus(isGoogler bool) *roller.AutoRollStatus {
	cleanIssue := func(issue *autoroll.AutoRollIssue) {
		// Clearing Issue and Subject out of an abundance of caution.
		issue.Issue = 0
		issue.Subject = ""
		issue.TryResults = nil
	}
	if isGoogler {
		cleanIssue = nil
	}
	status := a.status.Get(isGoogler, cleanIssue)
	status.ValidModes = []string{modes.MODE_RUNNING} // modeJsonHandler is not implemented.
	return status
}

// Return minimal status information for the bot.
func (a *AutoRoller) GetMiniStatus() *roller.AutoRollMiniStatus {
	return a.status.GetMini()
}

// UpdateStatus based on RecentRolls. errorMsg will be set unless preserveLastError is true.
func (a *AutoRoller) UpdateStatus(errorMsg string, preserveLastError bool) error {
	recent := a.recent.GetRecentRolls()
	numFailures := 0
	lastSuccessRev := ""
	for _, roll := range recent {
		if roll.Failed() {
			numFailures++
		} else if roll.Succeeded() {
			lastSuccessRev = roll.RollingTo
			break
		}
	}

	lastRoll := a.recent.LastRoll()
	lastRollRev := ""
	if lastRoll != nil {
		lastRollRev = lastRoll.RollingTo
	}

	commitsNotRolled := 0
	if lastSuccessRev != "" {
		headRev, err := a.childRepo.RevParse(a.childBranch)
		if err != nil {
			return err
		}
		revs, err := a.childRepo.RevList(headRev, "^"+lastSuccessRev)
		if err != nil {
			return err
		}
		commitsNotRolled = len(revs)
	}

	if preserveLastError {
		errorMsg = a.status.Get(true, nil).Error
	} else if errorMsg != "" {
		var lastRollIssue int64 = 0
		if lastRoll != nil {
			lastRollIssue = lastRoll.Issue
		}
		sklog.Warningf("Last roll %d; errorMsg: %s", lastRollIssue, errorMsg)
	}

	sklog.Infof("Updating status (%d)", commitsNotRolled)
	if err := a.status.Set(&roller.AutoRollStatus{
		AutoRollMiniStatus: roller.AutoRollMiniStatus{
			NumFailedRolls:      numFailures,
			NumNotRolledCommits: commitsNotRolled,
		},
		CurrentRoll:    a.recent.CurrentRoll(),
		Error:          errorMsg,
		FullHistoryUrl: "https://goto.google.com/skia-autoroll-history",
		IssueUrlBase:   "https://goto.google.com/skia-autoroll-cl/",
		LastRoll:       lastRoll,
		LastRollRev:    lastRollRev,
		Mode: &modes.ModeChange{
			Message: "https://sites.google.com/a/google.com/skia-infrastructure/docs/google3-autoroller",
			Mode:    modes.MODE_RUNNING,
			User:    "benjaminwagner@google.com",
			Time:    time.Date(2015, time.October, 14, 17, 6, 27, 0, time.UTC),
		},
		Recent: recent,
		Status: state_machine.S_NORMAL_ACTIVE,
	}); err != nil {
		return err
	}
	if lastRoll != nil && util.In(lastRoll.Result, []string{autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, autoroll.ROLL_RESULT_SUCCESS}) {
		a.liveness.ManualReset(lastRoll.Modified)
	}
	return nil
}

// AddOrUpdateIssue makes issue the current issue, handling any possible discrepancies due to
// missing previous requests. On error, returns an error safe for HTTP response.
func (a *AutoRoller) AddOrUpdateIssue(issue *autoroll.AutoRollIssue, method string) error {
	current := a.recent.CurrentRoll()

	// If we don't get an update to close the previous roll, close it automatically to avoid the error
	// "There is already an active roll. Cannot add another."
	if current != nil && current.Issue != issue.Issue {
		sklog.Warningf("Missing update to close %d. Closing automatically as failed.", current.Issue)
		current.Closed = true
		current.Result = autoroll.ROLL_RESULT_FAILURE
		if err := a.recent.Update(current); err != nil {
			sklog.Errorf("Failed to close current roll: %s", err)
			return errors.New("Failed to close current roll.")
		}
		current = nil
	}

	// If we don't see a roll until it's already closed, add it first to avoid the error "Cannot
	// insert a new roll which is already closed."
	if method == http.MethodPut && current == nil && issue.Closed {
		sklog.Warningf("Missing request to add %d before update marking closed. Automatically adding as in-progress.", issue.Issue)
		addIssue := new(autoroll.AutoRollIssue)
		*addIssue = *issue
		addIssue.Closed = false
		addIssue.Committed = false
		addIssue.Result = autoroll.ROLL_RESULT_IN_PROGRESS
		if err := a.recent.Add(addIssue); err != nil {
			sklog.Errorf("Failed to automatically add roll: %s", err)
			return errors.New("Failed to automatically add roll.")
		}
		current = a.recent.CurrentRoll()
	}

	if current == nil {
		if method != http.MethodPost {
			sklog.Warningf("Got %s instead of POST to add %d.", method, issue.Issue)
		}
		if err := a.recent.Add(issue); err != nil {
			sklog.Errorf("Failed to add roll: %s", err)
			return errors.New("Failed to add roll.")
		}
	} else {
		if method != http.MethodPut {
			sklog.Warningf("Got %s instead of PUT to update %d.", method, issue.Issue)
		}
		if err := a.recent.Update(issue); err != nil {
			sklog.Errorf("Failed to update roll: %s", err)
			return errors.New("Failed to update roll.")
		}
	}
	return nil
}

// Roll represents a Google3 AutoRoll attempt.
type Roll struct {
	ChangeListNumber jsonutils.Number `json:"changeListNumber"`
	// Closed indicates that the autoroller is finished with this CL. It does not correspond to any
	// property of the CL.
	Closed         bool           `json:"closed"`
	Created        jsonutils.Time `json:"created"`
	ErrorMsg       string         `json:"errorMsg"`
	Modified       jsonutils.Time `json:"modified"`
	Result         string         `json:"result"`
	RollingFrom    string         `json:"rollingFrom"`
	RollingTo      string         `json:"rollingTo"`
	Subject        string         `json:"subject"`
	Submitted      bool           `json:"submitted"`
	TestSummaryUrl string         `json:"testSummaryUrl"`
}

// AsIssue validates the Roll and generates an AutoRollIssue representing the same information. If
// invalid, returns an error safe for HTTP response.
func (roll Roll) AsIssue() (*autoroll.AutoRollIssue, error) {
	if util.TimeIsZero(time.Time(roll.Created)) || roll.RollingFrom == "" || roll.RollingTo == "" {
		return nil, errors.New("Missing parameter.")
	}
	if roll.Closed && roll.Result == autoroll.ROLL_RESULT_IN_PROGRESS {
		return nil, errors.New("Inconsistent parameters: result must be set.")
	}
	if roll.Submitted && !roll.Closed {
		return nil, errors.New("Inconsistent parameters: submitted but not closed.")
	}
	if !util.In(roll.Result, []string{autoroll.ROLL_RESULT_DRY_RUN_FAILURE, autoroll.ROLL_RESULT_IN_PROGRESS, autoroll.ROLL_RESULT_SUCCESS, autoroll.ROLL_RESULT_FAILURE}) {
		return nil, errors.New("Unsupported value for result.")
	}

	tryResults := []*autoroll.TryResult{}
	if roll.TestSummaryUrl != "" {
		url, err := url.Parse(roll.TestSummaryUrl)
		if err != nil {
			sklog.Warningf("Invalid Roll in request; invalid testSummaryUrl parameter %q: %s", roll.TestSummaryUrl, err)
			return nil, errors.New("Invalid testSummaryUrl parameter.")
		}
		testStatus := autoroll.TRYBOT_STATUS_STARTED
		testResult := ""
		switch roll.Result {
		case autoroll.ROLL_RESULT_DRY_RUN_FAILURE, autoroll.ROLL_RESULT_FAILURE:
			testStatus = autoroll.TRYBOT_STATUS_COMPLETED
			testResult = autoroll.TRYBOT_RESULT_FAILURE
		case autoroll.ROLL_RESULT_SUCCESS:
			testStatus = autoroll.TRYBOT_STATUS_COMPLETED
			testResult = autoroll.TRYBOT_RESULT_SUCCESS
		case autoroll.ROLL_RESULT_IN_PROGRESS:
		}
		tryResults = []*autoroll.TryResult{
			&autoroll.TryResult{
				Builder:  "Test Summary",
				Category: autoroll.TRYBOT_CATEGORY_CQ,
				Created:  time.Time(roll.Created),
				Result:   testResult,
				Status:   testStatus,
				Url:      url.String(),
			},
		}
	}
	return &autoroll.AutoRollIssue{
		Closed:      roll.Closed,
		Committed:   roll.Submitted,
		CommitQueue: !roll.Closed,
		Created:     time.Time(roll.Created),
		Issue:       int64(roll.ChangeListNumber),
		Modified:    time.Time(roll.Modified),
		Patchsets:   nil,
		Result:      roll.Result,
		RollingFrom: roll.RollingFrom,
		RollingTo:   roll.RollingTo,
		Subject:     roll.Subject,
		TryResults:  tryResults,
	}, nil
}

// rollHandler parses the JSON body as a Roll and inserts/updates it into the AutoRoll DB. The
// request must be authenticated via the protocol implemented in the webhook package. Use a POST
// request for a new roll and a PUT request to update an existing roll.
func (a *AutoRoller) rollHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed authentication.")
		return
	}
	roll := Roll{}
	if err := json.Unmarshal(data, &roll); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse request.")
		return
	}
	issue, err := roll.AsIssue()
	if err != nil {
		httputils.ReportError(w, r, nil, err.Error())
		return
	}
	if err := a.AddOrUpdateIssue(issue, r.Method); err != nil {
		httputils.ReportError(w, r, nil, err.Error())
		return
	}
	if err := a.UpdateStatus(roll.ErrorMsg, false); err != nil {
		httputils.ReportError(w, r, err, "Failed to set new status.")
		return
	}
}

// SetMode is not implemented for Google3 roller.
func (a *AutoRoller) SetMode(string, string, string) error {
	return errors.New("Not implemented for Google3 roller.")
}

// SetEmails is ignored for Google3 roller.
func (a *AutoRoller) SetEmails([]string) {}
