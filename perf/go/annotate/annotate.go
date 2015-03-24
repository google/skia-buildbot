package annotate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/activitylog"
	"go.skia.org/infra/perf/go/alerting"
	"go.skia.org/infra/perf/go/types"
)

// ISSUE_COMMENT_TEMPLATE is boilerplate text that is added when a new
// issue is created (through redirection in the UI). We convert the template
// to include the URL template that is used to link a cluster with an
// issue.
var ISSUE_COMMENT_TEMPLATE = fmt.Sprintf(`This bug was found via SkiaPerf.

Visit this URL to see the details of the suspicious cluster:

      %s

Don't remove the above URL, it is used to match bugs to alerts.
    `, alerting.TRACKED_ITEM_URL_TEMPLATE)

// Handler serves the /annotate/ endpoint for changing the status of an
// alert cluster. It also writes a new types.Activity log record to the database.
//
// Expects a POST of JSON of the following form:
//
//   {
//     Id: 20                - The id of the alerting cluster.
//     Status: "Ignore"      - The new Status value.
//     Message: "SKP Update" - The new Messge value.
//   }
//
// Returns JSON of the form:
//
//  {
//    "Bug": "http://"
//  }
//
// Where bug, if set, is the URL the user should be directed to to log a bug report.
func Handler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Annotate Handler: %q\n", r.URL.Path)

	if login.LoggedInAs(r) == "" {
		util.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to change an alert status.")
		return
	}
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}
	if r.Body == nil {
		util.ReportError(w, r, fmt.Errorf("Missing POST Body."), "POST with no request body.")
		return
	}

	req := struct {
		Id      int64
		Status  string
		Message string
	}{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		util.ReportError(w, r, err, "Unable to decode posted JSON.")
		return
	}

	if !util.In(req.Status, types.ValidStatusValues) {
		util.ReportError(w, r, fmt.Errorf("Invalid status value: %s", req.Status), "Unknown value.")
		return
	}

	// Store the updated values in the ClusterSummary.
	c, err := alerting.Get(req.Id)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load cluster summary.")
		return
	}
	c.Status = req.Status
	c.Message = req.Message
	if err := alerting.Write(c); err != nil {
		util.ReportError(w, r, err, "Failed to save cluster summary.")
		return
	}

	// Write a new Activity record.
	// TODO(jcgregorio) Move into alerting.Write().
	a := &types.Activity{
		UserID: login.LoggedInAs(r),
		Action: "Perf Alert: " + req.Status,
		URL:    fmt.Sprintf("https://skiaperf.com/cl/%d", req.Id),
	}
	if err := activitylog.Write(a); err != nil {
		util.ReportError(w, r, err, "Failed to save activity.")
		return
	}

	retval := map[string]string{}

	if req.Status == "Bug" {
		q := url.Values{
			"labels":  []string{"FromSkiaPerf,Type-Defect,Priority-Medium"},
			"comment": []string{fmt.Sprintf(ISSUE_COMMENT_TEMPLATE, req.Id)},
		}
		retval["Bug"] = "https://code.google.com/p/skia/issues/entry?" + q.Encode()
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(retval); err != nil {
		util.ReportError(w, r, err, "Error while encoding annotation response.")
	}
}
