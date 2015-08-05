/*
	Handlers and types specific to Chromium builds.
*/

package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/db"
	api "go.skia.org/infra/ct/go/frontend"
	skutil "go.skia.org/infra/go/util"
)

const (
	// URL returning the GIT commit hash of the last known good release of Chromium.
	CHROMIUM_LKGR_URL = "http://chromium-status.appspot.com/git-lkgr"
	// Base URL of the Chromium GIT repository, to be followed by commit hash.
	CHROMIUM_GIT_REPO_URL = "https://chromium.googlesource.com/chromium/src.git/+/"
	// URL of a base64-encoded file that includes the GIT commit hash last known good release of Skia.
	CHROMIUM_DEPS_FILE = "https://chromium.googlesource.com/chromium/src/+/master/DEPS?format=TEXT"
	// Base URL of the Skia GIT repository, to be followed by commit hash.
	SKIA_GIT_REPO_URL = "https://skia.googlesource.com/skia/+/"
)

var (
	chromiumBuildsTemplate           *template.Template = nil
	chromiumBuildRunsHistoryTemplate *template.Template = nil

	httpClient = skutil.NewTimeoutClient()
)

func reloadChromiumBuildTemplates() {
	chromiumBuildsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_builds.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
	chromiumBuildRunsHistoryTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/chromium_build_runs_history.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
	))
}

type ChromiumBuildDBTask struct {
	CommonCols

	ChromiumRev   string        `db:"chromium_rev"`
	ChromiumRevTs sql.NullInt64 `db:"chromium_rev_ts"`
	SkiaRev       string        `db:"skia_rev"`
}

func (task ChromiumBuildDBTask) GetTaskName() string {
	return "ChromiumBuild"
}

func (task ChromiumBuildDBTask) TableName() string {
	return db.TABLE_CHROMIUM_BUILD_TASKS
}

func (task ChromiumBuildDBTask) Select(query string, args ...interface{}) (interface{}, error) {
	result := []ChromiumBuildDBTask{}
	err := db.DB.Select(&result, query, args...)
	return result, err
}

func chromiumBuildsView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumBuildsTemplate, w, r)
}

type AddChromiumBuildTaskVars struct {
	AddTaskCommonVars

	ChromiumRev   string `json:"chromium_rev"`
	ChromiumRevTs string `json:"chromium_rev_ts"`
	SkiaRev       string `json:"skia_rev"`
}

func (task *AddChromiumBuildTaskVars) GetInsertQueryAndBinds() (string, []interface{}, error) {
	if task.ChromiumRev == "" ||
		task.SkiaRev == "" ||
		task.ChromiumRevTs == "" {
		return "", nil, fmt.Errorf("Invalid parameters")
	}
	// Example timestamp format: "Wed Jul 15 13:42:19 2015"
	parsedTs, err := time.Parse(time.ANSIC, task.ChromiumRevTs)
	if err != nil {
		return "", nil, err
	}
	chromiumRevTs := parsedTs.UTC().Format("20060102150405")
	return fmt.Sprintf("INSERT INTO %s (username,chromium_rev,chromium_rev_ts,skia_rev,ts_added,repeat_after_days) VALUES (?,?,?,?,?,?);",
			db.TABLE_CHROMIUM_BUILD_TASKS),
		[]interface{}{
			task.Username,
			task.ChromiumRev,
			chromiumRevTs,
			task.SkiaRev,
			task.TsAdded,
			task.RepeatAfterDays,
		},
		nil
}

func addChromiumBuildTaskHandler(w http.ResponseWriter, r *http.Request) {
	addTaskHandler(w, r, &AddChromiumBuildTaskVars{})
}

func getRevDataHandler(getLkgr func() (string, error), gitRepoUrl string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	revString := r.FormValue("rev")
	if revString == "" {
		skutil.ReportError(w, r, fmt.Errorf("No revision specified"), "")
		return
	}

	if strings.EqualFold(revString, "LKGR") {
		lkgr, err := getLkgr()
		if err != nil {
			skutil.ReportError(w, r, fmt.Errorf("Unable to retrieve LKGR revision"), "")
		}
		glog.Infof("Retrieved LKGR commit hash: %s", lkgr)
		revString = lkgr
	}
	commitUrl := gitRepoUrl + revString + "?format=JSON"
	glog.Infof("Reading revision detail from %s", commitUrl)
	resp, err := httpClient.Get(commitUrl)
	if err != nil {
		skutil.ReportError(w, r, err, "Unable to retrieve revision detail")
		return
	}
	defer skutil.Close(resp.Body)
	if resp.StatusCode == 404 {
		// Return successful empty response, since the user could be typing.
		if err := json.NewEncoder(w).Encode(map[string]interface{}{}); err != nil {
			skutil.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
			return
		}
		return
	}
	if resp.StatusCode != 200 {
		skutil.ReportError(w, r, fmt.Errorf("Unable to retrieve revision detail"), "")
		return
	}
	// Remove junk in the first line. https://code.google.com/p/gitiles/issues/detail?id=31
	bufBody := bufio.NewReader(resp.Body)
	if _, err = bufBody.ReadString('\n'); err != nil {
		skutil.ReportError(w, r, err, "Unable to retrieve revision detail")
		return
	}
	if _, err = io.Copy(w, bufBody); err != nil {
		skutil.ReportError(w, r, err, "Unable to retrieve revision detail")
		return
	}
}

func getChromiumLkgr() (string, error) {
	glog.Infof("Reading Chromium LKGR from %s", CHROMIUM_LKGR_URL)
	resp, err := httpClient.Get(CHROMIUM_LKGR_URL)
	if err != nil {
		return "", err
	}
	defer skutil.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return bytes.NewBuffer(body).String(), nil
}

func getChromiumRevDataHandler(w http.ResponseWriter, r *http.Request) {
	getRevDataHandler(getChromiumLkgr, CHROMIUM_GIT_REPO_URL, w, r)
}

var skiaRevisionRegexp = regexp.MustCompile("'skia_revision': '([0-9a-fA-F]{2,40})'")

// Copied from https://github.com/google/skia-buildbot/blob/016cce36f0cd487c9586b013979705e49dd76f8e/appengine_scripts/skia-tree-status/status.py#L178
// to work around 403 error.
func getSkiaLkgr() (string, error) {
	glog.Infof("Reading Skia LKGR from %s", CHROMIUM_DEPS_FILE)
	resp, err := httpClient.Get(CHROMIUM_DEPS_FILE)
	if err != nil {
		return "", err
	}
	defer skutil.Close(resp.Body)
	decodedBody, err := ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, resp.Body))
	if err != nil {
		return "", err
	}
	regexpMatches := skiaRevisionRegexp.FindSubmatch(decodedBody)
	if regexpMatches == nil || len(regexpMatches) < 2 || len(regexpMatches[1]) == 0 {
		return "", fmt.Errorf("Could not find skia_revision in %s", CHROMIUM_DEPS_FILE)
	}
	return bytes.NewBuffer(regexpMatches[1]).String(), nil
}

func getSkiaRevDataHandler(w http.ResponseWriter, r *http.Request) {
	getRevDataHandler(getSkiaLkgr, SKIA_GIT_REPO_URL, w, r)
}

// Define api.ChromiumBuildUpdateVars in this package so we can add methods.
type ChromiumBuildUpdateVars struct {
	api.ChromiumBuildUpdateVars
}

func (task *ChromiumBuildUpdateVars) GetUpdateExtraClausesAndBinds() ([]string, []interface{}, error) {
	return nil, nil, nil
}

func updateChromiumBuildTaskHandler(w http.ResponseWriter, r *http.Request) {
	updateTaskHandler(&ChromiumBuildUpdateVars{}, db.TABLE_CHROMIUM_BUILD_TASKS, w, r)
}

func deleteChromiumBuildTaskHandler(w http.ResponseWriter, r *http.Request) {
	deleteTaskHandler(&ChromiumBuildDBTask{}, w, r)
}

func chromiumBuildRunsHistoryView(w http.ResponseWriter, r *http.Request) {
	executeSimpleTemplate(chromiumBuildRunsHistoryTemplate, w, r)
}

func getChromiumBuildTasksHandler(w http.ResponseWriter, r *http.Request) {
	getTasksHandler(&ChromiumBuildDBTask{}, w, r)
}

// Validate that the given chromiumBuild exists in the DB.
func validateChromiumBuild(chromiumBuild ChromiumBuildDBTask) error {
	buildCount := []int{}
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE chromium_rev = ? AND skia_rev = ? AND ts_completed IS NOT NULL AND failure = 0", db.TABLE_CHROMIUM_BUILD_TASKS)
	if err := db.DB.Select(&buildCount, query, chromiumBuild.ChromiumRev, chromiumBuild.SkiaRev); err != nil || len(buildCount) < 1 || buildCount[0] == 0 {
		glog.Info(err)
		return fmt.Errorf("Unable to validate chromium_build parameter %v", chromiumBuild)
	}
	return nil
}
