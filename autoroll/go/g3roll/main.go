/*
	Keep track of Skia rolls into Google3.

  Rolls are POSTed to this application via webhook.
*/

package main

import (
	"encoding/json"
	"flag"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"

	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/webhook"
)

var (
	mainTemplate *template.Template = nil
)

// flags
var (
	childName    = flag.String("childName", "Skia into Google3", "Name of the project to roll.")
	childPath    = flag.String("childPath", "HEAD", "Subdirectory into which to roll.")
	childBranch  = flag.String("child_branch", "master", "Branch of the project we want to roll.")
	host         = flag.String("host", "localhost", "HTTP service host")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port         = flag.String("port", ":8000", "HTTP service port (e.g., ':8000')")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	useMetadata  = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	workdir      = flag.String("workdir", ".", "Directory to use for scratch work.")
)

func reloadTemplates() {
	// Change the current working directory to two directories up from this source file so that we
	// can read templates and serve static (res/) files.

	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	mainTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/main.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func Init() {
	reloadTemplates()
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Don't use cached templates in testing mode.
	if *local {
		reloadTemplates()
	}
	mainPage := struct {
		ProjectName string
		ProjectUser string
	}{
		ProjectName: *childName,
		ProjectUser: "",
	}
	if err := mainTemplate.Execute(w, mainPage); err != nil {
		sklog.Errorln("Failed to expand template:", err)
	}
}

type AutoRoller struct {
	recent    *recent_rolls.RecentRolls
	status    *roller.AutoRollStatusCache
	childRepo *git.Repo
}

func NewAutoRoller(workdir string) (*AutoRoller, error) {
	recent, err := recent_rolls.NewRecentRolls(path.Join(workdir, "recent_rolls.bdb"))
	if err != nil {
		return nil, err
	}

	childRepo, err := git.NewRepo(common.REPO_SKIA, path.Join(workdir, "repo"))
	if err != nil {
		return nil, err
	}

	a := &AutoRoller{
		recent:    recent,
		status:    &roller.AutoRollStatusCache{},
		childRepo: childRepo,
	}

	if err := a.SetStatus(""); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *AutoRoller) GetStatus(isGoogler bool) *roller.AutoRollStatus {
	cleanIssue := func(issue *autoroll.AutoRollIssue) {
		issue.Issue = rand.Int63()
		issue.Subject = ""
		issue.TryResults = nil
	}
	if isGoogler {
		cleanIssue = nil
	}
	return a.status.Get(isGoogler, cleanIssue)
}

func (a *AutoRoller) SetStatus(errorMsg string) error {
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
		if err := a.childRepo.Update(); err != nil {
			return err
		}
		headRev, err := a.childRepo.RevParse(*childBranch)
		if err != nil {
			return err
		}
		revs, err := a.childRepo.RevList(headRev, "^"+lastSuccessRev)
		if err != nil {
			return err
		}
		commitsNotRolled = len(revs)
	}

	sklog.Infof("Updating status (%d)", commitsNotRolled)
	return a.status.Set(&roller.AutoRollStatus{
		AutoRollMiniStatus: roller.AutoRollMiniStatus{
			NumFailedRolls:      numFailures,
			NumNotRolledCommits: commitsNotRolled,
		},
		CurrentRoll: a.recent.CurrentRoll(),
		Error:       errorMsg,
		GerritUrl:   "",
		LastRoll:    a.recent.LastRoll(),
		LastRollRev: lastRollRev,
		Mode: &modes.ModeChange{
			Message: "https://sites.google.com/a/google.com/skia-infrastructure/docs/google3-autoroller",
			Mode:    modes.MODE_RUNNING,
			User:    "benjaminwagner@google.com",
			Time:    time.Date(2015, time.October, 14, 17, 6, 27, 0, time.UTC),
		},
		Recent: recent,
		Status: state_machine.S_NORMAL_ACTIVE,
	})
}

func (a *AutoRoller) modeJsonHandler(w http.ResponseWriter, r *http.Request) {
	httputils.ReportError(w, r, nil, "Not implemented for Google3 roller.")
}

func (a *AutoRoller) statusJsonHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// Obtain the status info. Only display error messages if the user
	// is a logged-in Googler.
	status := a.GetStatus(login.IsGoogler(r))
	status.ValidModes = []string{modes.MODE_RUNNING} // modeJsonHandler is not implemented.
	if err := json.NewEncoder(w).Encode(&status); err != nil {
		sklog.Error(err)
	}
}

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

// rollHandler parses the JSON body as a Roll and inserts/updates it into the AutoRoll DB. The
// request must be authenticated via the protocol implemented in the webhook package.
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
	if util.TimeIsZero(time.Time(roll.Created)) || roll.RollingFrom == "" || roll.RollingTo == "" {
		httputils.ReportError(w, r, nil, "Missing parameter.")
		return
	}
	if roll.Closed && roll.Result == "" {
		httputils.ReportError(w, r, nil, "Missing parameter: result must be set.")
		return
	}
	if roll.Submitted && !roll.Closed {
		httputils.ReportError(w, r, nil, "Inconsistent parameters: submitted but not closed.")
		return
	}
	if !util.In(roll.Result, []string{autoroll.ROLL_RESULT_DRY_RUN_FAILURE, autoroll.ROLL_RESULT_IN_PROGRESS, autoroll.ROLL_RESULT_SUCCESS, autoroll.ROLL_RESULT_FAILURE}) {
		httputils.ReportError(w, r, nil, "Unsupported value for result.")
		return
	}

	tryResults := []*autoroll.TryResult{}
	if roll.TestSummaryUrl != "" {
		url, err := url.Parse(roll.TestSummaryUrl)
		if err != nil {
			httputils.ReportError(w, r, err, "Invalid testResultsLink parameter.")
			return
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
				Builder:  "summary",
				Category: autoroll.TRYBOT_CATEGORY_CQ,
				Created:  time.Time(roll.Created),
				Result:   testResult,
				Status:   testStatus,
				Url:      url.String(),
			},
		}
	}
	issue := autoroll.AutoRollIssue{
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
	}
	if r.Method == http.MethodPost {
		if err := a.recent.Add(&issue); err != nil {
			httputils.ReportError(w, r, err, "Failed to add roll.")
			return
		}
	} else {
		if err := a.recent.Update(&issue); err != nil {
			httputils.ReportError(w, r, err, "Failed to update roll.")
			return
		}
	}
	if err := a.SetStatus(roll.ErrorMsg); err != nil {
		httputils.ReportError(w, r, err, "Failed to set new status.")
		return
	}
}

func runServer(arb *AutoRoller) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/json/mode", arb.modeJsonHandler).Methods(http.MethodPost)
	r.HandleFunc("/json/status", httputils.CorsHandler(arb.statusJsonHandler))
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/json/roll", arb.rollHandler).Methods(http.MethodPost, http.MethodPut)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	common.InitWithMust(
		"autoroll",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	Init()

	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	if *local {
		*useMetadata = false
	}

	arb, err := NewAutoRoller(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Feed AutoRoll stats into metrics.
	go func() {
		for range time.Tick(time.Minute) {
			status := arb.GetStatus(false)
			v := int64(0)
			if status.LastRoll != nil && status.LastRoll.Closed && status.LastRoll.Committed {
				v = int64(1)
			}
			metrics2.GetInt64Metric("autoroll.last-roll-result", map[string]string{"child_path": *childPath}).Update(v)
		}
	}()

	login.SimpleInitMust(*port, *local)

	runServer(arb)
}
