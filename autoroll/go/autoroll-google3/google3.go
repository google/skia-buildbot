/*
	Keep track of Skia rolls into Google3.

	Rolls are added/updated by POST/PUT request with webhook authentication.
*/

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/webhook"
	"golang.org/x/oauth2"
)

const (
	ISSUE_URL_BASE = "https://goto.google.com/skia-autoroll-cl/"
)

// AutoRoller provides a handler for adding/updating Rolls, translating them into AutoRollIssue for
// storage in RecentRolls. It also manages an AutoRollStatusCache for status handlers.
type AutoRoller struct {
	cfg         *roller.AutoRollerConfig
	recent      *recent_rolls.RecentRolls
	status      *status.Cache
	childBranch string
	childRepo   *gitiles.Repo
	mtx         sync.Mutex
	liveness    metrics2.Liveness
}

func NewAutoRoller(ctx context.Context, cfg *roller.AutoRollerConfig, client *http.Client, ts oauth2.TokenSource) (*AutoRoller, error) {
	recent, err := recent_rolls.NewRecentRolls(ctx, cfg.RollerName)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	statusDB := status.NewDatastoreDB()
	cache, err := status.NewCache(ctx, statusDB, cfg.RollerName)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	a := &AutoRoller{
		cfg:         cfg,
		recent:      recent,
		status:      cache,
		childBranch: cfg.Google3RepoManager.ChildBranch,
		childRepo:   gitiles.NewRepo(cfg.Google3RepoManager.ChildRepo, client),
		liveness:    metrics2.NewLiveness("last_autoroll_landed"),
	}

	if err := a.UpdateStatus(ctx, "", true); err != nil {
		return nil, skerr.Wrap(err)
	}
	return a, nil
}

// Start ensures DBs are closed when ctx is canceled.
func (a *AutoRoller) Start(ctx context.Context, tickFrequency, repoFrequency time.Duration) {
	go cleanup.Repeat(repoFrequency, func(ctx context.Context) {
		util.LogErr(a.UpdateStatus(ctx, "", true))
	}, nil)
}

func (a *AutoRoller) AddHandlers(r *mux.Router) {
	r.HandleFunc("/json/roll", a.rollHandler).Methods(http.MethodPost, http.MethodPut)
}

// UpdateStatus based on RecentRolls. errorMsg will be set unless preserveLastError is true.
func (a *AutoRoller) UpdateStatus(ctx context.Context, errorMsg string, preserveLastError bool) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	lastStatus := a.status.Get()

	recent := a.recent.GetRecentRolls()
	numFailures := 0
	lastSuccessRev := ""
	for _, roll := range recent {
		if roll.Succeeded() {
			lastSuccessRev = roll.RollingTo
			break
		}
		if lastStatus != nil && lastStatus.LastRoll != nil && lastStatus.LastRoll.Issue == roll.Issue {
			numFailures += lastStatus.AutoRollMiniStatus.NumFailedRolls
			lastSuccessRev = lastStatus.LastRollRev
			break
		}
		if roll.Failed() {
			numFailures++
		}
	}

	commitsNotRolled := 0
	if lastSuccessRev != "" {
		headRev, err := a.childRepo.Details(ctx, a.childBranch)
		if err != nil {
			return err
		}
		revs, err := a.childRepo.LogFirstParent(ctx, lastSuccessRev, headRev.Hash)
		if err != nil {
			return err
		}
		commitsNotRolled = len(revs)
	}

	lastRoll := a.recent.LastRoll()

	if preserveLastError {
		if lastStatus != nil {
			errorMsg = lastStatus.Error
		}
	} else if errorMsg != "" {
		var lastRollIssue int64 = 0
		if lastRoll != nil {
			lastRollIssue = lastRoll.Issue
		}
		sklog.Warningf("Last roll %d; errorMsg: %s", lastRollIssue, errorMsg)
	}

	currentRollRev := ""
	currentRoll := a.recent.CurrentRoll()
	if currentRoll != nil {
		currentRollRev = currentRoll.RollingTo
	}
	newStatus := &status.AutoRollStatus{
		AutoRollMiniStatus: status.AutoRollMiniStatus{
			CurrentRollRev:      currentRollRev,
			LastRollRev:         lastSuccessRev,
			Mode:                modes.ModeRunning,
			NumFailedRolls:      numFailures,
			NumNotRolledCommits: commitsNotRolled,
		},
		ChildName:       a.cfg.ChildDisplayName,
		CurrentRoll:     a.recent.CurrentRoll(),
		Error:           errorMsg,
		FullHistoryUrl:  "https://goto.google.com/skia-autoroll-history",
		IssueUrlBase:    ISSUE_URL_BASE,
		LastRoll:        lastRoll,
		ParentName:      a.cfg.ParentDisplayName,
		Recent:          recent,
		Status:          state_machine.S_NORMAL_ACTIVE,
		ValidModes:      []string{modes.ModeRunning},
		ValidStrategies: []string{strategy.ROLL_STRATEGY_BATCH},
	}
	sklog.Infof("Updating status: %+v", newStatus)
	if err := a.status.Set(ctx, a.cfg.RollerName, newStatus); err != nil {
		return err
	}
	if lastRoll != nil && util.In(lastRoll.Result, []string{autoroll.ROLL_RESULT_DRY_RUN_SUCCESS, autoroll.ROLL_RESULT_SUCCESS}) {
		a.liveness.ManualReset(lastRoll.Modified)
	}
	return a.status.Update(ctx)
}

// AddOrUpdateIssue makes issue the current issue, handling any possible discrepancies due to
// missing previous requests. On error, returns an error safe for HTTP response.
func (a *AutoRoller) AddOrUpdateIssue(ctx context.Context, issue *autoroll.AutoRollIssue, method string) error {
	current := a.recent.CurrentRoll()

	// If we don't get an update to close the previous roll, close it automatically to avoid the error
	// "There is already an active roll. Cannot add another."
	if current != nil && current.Issue != issue.Issue {
		sklog.Warningf("Missing update to close %d. Closing automatically as failed.", current.Issue)
		current.Closed = true
		current.Result = autoroll.ROLL_RESULT_FAILURE
		if err := a.recent.Update(ctx, current); err != nil {
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
		if err := a.recent.Add(ctx, addIssue); err != nil {
			sklog.Errorf("Failed to automatically add roll: %s", err)
			return errors.New("Failed to automatically add roll.")
		}
		current = a.recent.CurrentRoll()
	}

	if current == nil {
		if method != http.MethodPost {
			sklog.Warningf("Got %s instead of POST to add %d.", method, issue.Issue)
		}
		if err := a.recent.Add(ctx, issue); err != nil {
			sklog.Errorf("Failed to add roll: %s", err)
			return errors.New("Failed to add roll.")
		}
	} else {
		if method != http.MethodPut {
			sklog.Warningf("Got %s instead of PUT to update %d.", method, issue.Issue)
		}
		if err := a.recent.Update(ctx, issue); err != nil {
			sklog.Errorf("Failed to update roll: %s", err)
			return errors.New("Failed to update roll.")
		}
	}
	return nil
}

// Roll represents a Google3 AutoRoll attempt.
type Roll struct {
	ChangeListNumber jsonutils.Number `json:"changeListNumber"`
	CheckResults     []*CheckResult   `json:"checkResults"`
	// Closed indicates that the autoroller is finished with this CL. It does not correspond to any
	// property of the CL.
	Closed      bool           `json:"closed"`
	Created     jsonutils.Time `json:"created"`
	ErrorMsg    string         `json:"errorMsg"`
	Modified    jsonutils.Time `json:"modified"`
	Result      string         `json:"result"`
	RollingFrom string         `json:"rollingFrom"`
	RollingTo   string         `json:"rollingTo"`
	Subject     string         `json:"subject"`
	Submitted   bool           `json:"submitted"`
	// Deprecated.
	TestSummaryUrl string `json:"testSummaryUrl"`
}

// CheckResult represents a Google3 CL presubmit check.
type CheckResult struct {
	Name      string         `json:"name"`
	Result    string         `json:"result"`
	Status    string         `json:"status"`
	StartTime jsonutils.Time `json:"startTime"`
	Url       string         `json:"url"`
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

	isDryRun := false
	cqFinished := false
	cqSuccess := false
	dryRunFinished := false
	dryRunSuccess := false
	switch roll.Result {
	case autoroll.ROLL_RESULT_DRY_RUN_SUCCESS:
		isDryRun = true
		dryRunFinished = true
		dryRunSuccess = true
	case autoroll.ROLL_RESULT_DRY_RUN_FAILURE:
		isDryRun = true
		dryRunFinished = true
	case autoroll.ROLL_RESULT_DRY_RUN_IN_PROGRESS:
		isDryRun = true
	case autoroll.ROLL_RESULT_SUCCESS:
		cqFinished = true
		cqSuccess = true
	case autoroll.ROLL_RESULT_FAILURE:
		cqFinished = true
	}

	tryResults := []*autoroll.TryResult{}
	// TestSummaryUrl is for legacy requests that do not specify CheckResults.
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
			{
				Builder:  "Test Summary",
				Category: autoroll.TRYBOT_CATEGORY_CQ,
				Created:  time.Time(roll.Created),
				Result:   testResult,
				Status:   testStatus,
				Url:      url.String(),
			},
		}
	}
	for _, r := range roll.CheckResults {
		url, _ := url.Parse(fmt.Sprintf("%s%d", ISSUE_URL_BASE, roll.ChangeListNumber))
		if r.Url != "" {
			var err error
			url, err = url.Parse(r.Url)
			if err != nil {
				sklog.Warningf("Invalid Roll in request; invalid checkResults.url parameter %q: %s", r.Url, err)
				return nil, errors.New("Invalid checkResult.url parameter.")
			}
		}
		if !util.In(r.Status, []string{autoroll.TRYBOT_STATUS_STARTED, autoroll.TRYBOT_STATUS_COMPLETED, autoroll.TRYBOT_STATUS_SCHEDULED}) {
			return nil, errors.New("Unsupported value for checkResult.status.")
		}
		result := r.Result
		if r.Status == autoroll.TRYBOT_STATUS_COMPLETED {
			if !util.In(result, []string{autoroll.TRYBOT_RESULT_CANCELED, autoroll.TRYBOT_RESULT_SUCCESS, autoroll.TRYBOT_RESULT_FAILURE}) {
				return nil, errors.New("Unsupported value for checkResult.result.")
			}
		} else {
			result = ""
		}
		tryResults = append(tryResults, &autoroll.TryResult{
			Builder:  r.Name,
			Category: autoroll.TRYBOT_CATEGORY_CQ,
			Created:  time.Time(r.StartTime),
			Result:   result,
			Status:   r.Status,
			Url:      url.String(),
		})
	}
	return &autoroll.AutoRollIssue{
		Closed:         roll.Closed,
		Committed:      roll.Submitted,
		Created:        time.Time(roll.Created),
		CqFinished:     cqFinished,
		CqSuccess:      cqSuccess,
		DryRunFinished: dryRunFinished,
		DryRunSuccess:  dryRunSuccess,
		IsDryRun:       isDryRun,
		Issue:          int64(roll.ChangeListNumber),
		Modified:       time.Time(roll.Modified),
		Patchsets:      nil,
		Result:         roll.Result,
		RollingFrom:    roll.RollingFrom,
		RollingTo:      roll.RollingTo,
		Subject:        roll.Subject,
		TryResults:     tryResults,
	}, nil
}

// rollHandler parses the JSON body as a Roll and inserts/updates it into the AutoRoll DB. The
// request must be authenticated via the protocol implemented in the webhook package. Use a POST
// request for a new roll and a PUT request to update an existing roll.
func (a *AutoRoller) rollHandler(w http.ResponseWriter, r *http.Request) {
	data, err := webhook.AuthenticateRequest(r)
	if err != nil {
		httputils.ReportError(w, err, "Failed authentication.", http.StatusInternalServerError)
		return
	}
	roll := Roll{}
	if err := json.Unmarshal(data, &roll); err != nil {
		httputils.ReportError(w, err, "Failed to parse request.", http.StatusInternalServerError)
		return
	}
	issue, err := roll.AsIssue()
	if err != nil {
		httputils.ReportError(w, nil, err.Error(), http.StatusInternalServerError)
		return
	}
	ctx := context.Background()
	if err := a.AddOrUpdateIssue(ctx, issue, r.Method); err != nil {
		httputils.ReportError(w, nil, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := a.UpdateStatus(ctx, roll.ErrorMsg, false); err != nil {
		httputils.ReportError(w, err, "Failed to set new status.", http.StatusInternalServerError)
		return
	}
}
