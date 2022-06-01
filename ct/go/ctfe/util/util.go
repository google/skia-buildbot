/*
	Utility functions used by all of ctfe.
*/

package util

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
)

// URIs for frontend handlers.
const (
	CHROMIUM_PERF_URI                  = "chromium_perf/"
	CHROMIUM_PERF_RUNS_URI             = "chromium_perf_runs/"
	ADD_CHROMIUM_PERF_TASK_POST_URI    = "_/add_chromium_perf_task"
	GET_CHROMIUM_PERF_TASKS_POST_URI   = "_/get_chromium_perf_tasks"
	DELETE_CHROMIUM_PERF_TASK_POST_URI = "_/delete_chromium_perf_task"
	REDO_CHROMIUM_PERF_TASK_POST_URI   = "_/redo_chromium_perf_task"
	EDIT_CHROMIUM_PERF_TASK_POST_URI   = "_/edit_chromium_perf_task"

	CHROMIUM_ANALYSIS_URI                  = "chromium_analysis/"
	CHROMIUM_ANALYSIS_RUNS_URI             = "chromium_analysis_runs/"
	ADD_CHROMIUM_ANALYSIS_TASK_POST_URI    = "_/add_chromium_analysis_task"
	GET_CHROMIUM_ANALYSIS_TASKS_POST_URI   = "_/get_chromium_analysis_tasks"
	DELETE_CHROMIUM_ANALYSIS_TASK_POST_URI = "_/delete_chromium_analysis_task"
	REDO_CHROMIUM_ANALYSIS_TASK_POST_URI   = "_/redo_chromium_analysis_task"
	EDIT_CHROMIUM_ANALYSIS_TASK_POST_URI   = "_/edit_chromium_analysis_task"

	METRICS_ANALYSIS_URI                  = "metrics_analysis/"
	METRICS_ANALYSIS_RUNS_URI             = "metrics_analysis_runs/"
	ADD_METRICS_ANALYSIS_TASK_POST_URI    = "_/add_metrics_analysis_task"
	GET_METRICS_ANALYSIS_TASKS_POST_URI   = "_/get_metrics_analysis_tasks"
	DELETE_METRICS_ANALYSIS_TASK_POST_URI = "_/delete_metrics_analysis_task"
	REDO_METRICS_ANALYSIS_TASK_POST_URI   = "_/redo_metrics_analysis_task"

	ADMIN_TASK_URI = "admin_tasks/"

	RECREATE_PAGE_SETS_RUNS_URI             = "recreate_page_sets_runs/"
	ADD_RECREATE_PAGE_SETS_TASK_POST_URI    = "_/add_recreate_page_sets_task"
	GET_RECREATE_PAGE_SETS_TASKS_POST_URI   = "_/get_recreate_page_sets_tasks"
	DELETE_RECREATE_PAGE_SETS_TASK_POST_URI = "_/delete_recreate_page_sets_task"
	REDO_RECREATE_PAGE_SETS_TASK_POST_URI   = "_/redo_recreate_page_sets_task"

	RECREATE_WEBPAGE_ARCHIVES_RUNS_URI             = "recreate_webpage_archives_runs/"
	ADD_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI    = "_/add_recreate_webpage_archives_task"
	GET_RECREATE_WEBPAGE_ARCHIVES_TASKS_POST_URI   = "_/get_recreate_webpage_archives_tasks"
	DELETE_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI = "_/delete_recreate_webpage_archives_task"
	REDO_RECREATE_WEBPAGE_ARCHIVES_TASK_POST_URI   = "_/redo_recreate_webpage_archives_task"

	RUNS_HISTORY_URI = "history/"

	PENDING_TASKS_URI = "queue/"

	PAGE_SETS_PARAMETERS_POST_URI = "_/page_sets/"
	CL_DATA_POST_URI              = "_/cl_data"
	BENCHMARKS_PLATFORMS_POST_URI = "_/benchmarks_platforms/"
	TASK_PRIORITIES_GET_URI       = "_/task_priorities/"
	IS_ADMIN_GET_URI              = "_/is_admin/"
	COMPLETED_TASKS_POST_URL      = "_/completed_tasks"

	RESULTS_URI = "/results/"

	OAUTH2_CALLBACK_PATH = "/oauth2callback/"

	MAX_GROUPNAME_LEN = 30
)

var DomainsWithViewAccess = []string{"google.com"}

// Function to run before executing a template.
var PreExecuteTemplateHook = func() {}

func UserHasEditRights(r *http.Request) bool {
	return strings.HasSuffix(login.LoggedInAs(r), "@google.com")
}

func UserHasAdminRights(r *http.Request) bool {
	return UserHasEditRights(r) && login.IsAdmin(r)
}

func ExecuteSimpleTemplate(template *template.Template, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	PreExecuteTemplateHook()
	if err := template.Execute(w, struct{}{}); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to expand template: %v", err), http.StatusInternalServerError)
		return
	}
}

func GetQualifiedCustomWebpages(customWebpages, benchmarkArgs string) ([]string, error) {
	qualifiedWebpages := []string{}
	if customWebpages != "" {
		if !strings.Contains(benchmarkArgs, util.USE_LIVE_SITES_FLAGS) {
			return nil, errors.New("Cannot use custom webpages without " + util.USE_LIVE_SITES_FLAGS)
		}
		r := csv.NewReader(strings.NewReader(customWebpages))
		for {
			records, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			for _, record := range records {
				if strings.TrimSpace(record) == "" {
					// Skip empty webpages.
					continue
				}
				var qualifiedWebpage string
				if strings.HasPrefix(record, "http://") || strings.HasPrefix(record, "https://") {
					qualifiedWebpage = record
				} else if len(strings.Split(record, ".")) > 2 {
					qualifiedWebpage = fmt.Sprintf("http://%s", record)
				} else {
					qualifiedWebpage = fmt.Sprintf("http://www.%s", record)
				}
				qualifiedWebpages = append(qualifiedWebpages, qualifiedWebpage)
			}
		}
	}
	return qualifiedWebpages, nil
}

// Returns true if the string is non-empty, unless strconv.ParseBool parses the string as false.
func ParseBoolFormValue(string string) bool {
	if string == "" {
		return false
	} else if val, err := strconv.ParseBool(string); val == false && err == nil {
		return false
	} else {
		return true
	}
}
