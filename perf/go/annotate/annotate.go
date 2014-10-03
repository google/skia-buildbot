package annotate

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/login"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/activitylog"
	"skia.googlesource.com/buildbot.git/perf/go/alerting"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

// Handler serves the /annotate/ endpoint for changing the status of an
// alert cluster. It also writes a new types.Activity log record to the database.
//
// Expects a POST'd form with the following values:
//
//   id - The id of the alerting cluster.
//   status - The new Status value.
//   message - The new Messge value.
//
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
	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Failed to parse query params.")
		return
	}

	// Load the form data.
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 32)
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("id parameter must be an integer %s.", r.FormValue("id")))
		return
	}
	newStatus := r.FormValue("status")
	message := r.FormValue("message")
	if !util.In(newStatus, types.ValidStatusValues) {
		util.ReportError(w, r, fmt.Errorf("Invalid status value: %s", newStatus), "Unknown value.")
		return
	}

	// Store the updated values in the ClusterSummary.
	c, err := alerting.Get(id)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load cluster summary.")
		return
	}
	c.Status = newStatus
	c.Message = message
	if err := alerting.Write(c); err != nil {
		util.ReportError(w, r, err, "Failed to save cluster summary.")
		return
	}

	// Write a new Activity record.
	a := &types.Activity{
		UserID: login.LoggedInAs(r),
		Action: "Perf Alert: " + newStatus,
		URL:    fmt.Sprintf("http://skiaperf.com/cl/%d", id),
	}
	if err := activitylog.Write(a); err != nil {
		util.ReportError(w, r, err, "Failed to save activity.")
		return
	}

	if newStatus != "Bug" {
		http.Redirect(w, r, "/alerts/", 303)
	} else {
		q := url.Values{
			"labels": []string{"FromSkiaPerf,Type-Defect,Priority-Medium"},
			"comment": []string{fmt.Sprintf(`This bug was found via SkiaPerf.

Visit this URL to see the details of the suspicious cluster:

      http://skiaperf.com/cl/%d.

Don't remove the above URL, it is used to match bugs to alerts.
    `, id)},
		}
		codesiteURL := "https://code.google.com/p/skia/issues/entry?" + q.Encode()
		http.Redirect(w, r, codesiteURL, http.StatusTemporaryRedirect)
	}
}
