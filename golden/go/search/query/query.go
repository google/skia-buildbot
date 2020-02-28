// Package query contains the logic involving parsing queries to
// Gold's search endpoints.
package query

import (
	"net/http"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

const (
	// SortAscending indicates that we want to sort in ascending order.
	SortAscending = "asc"

	// SortDescending indicates that we want to sort in descending order.
	SortDescending = "desc"
)

// ParseSearch parses the request parameters from the URL query string or from the
// form parameters and stores the parsed and validated values in query.
func ParseSearch(r *http.Request, q *Search) error {
	if err := r.ParseForm(); err != nil {
		return skerr.Wrapf(err, "parsing form")
	}

	// Parse the list of fields that need to match and ensure the
	// test name is in it.
	var ok bool
	if q.Match, ok = r.Form["match"]; ok {
		if !util.In(types.PrimaryKeyField, q.Match) {
			q.Match = append(q.Match, types.PrimaryKeyField)
		}
	} else {
		q.Match = []string{types.PrimaryKeyField}
	}

	validate := shared.Validation{}

	// Parse the query strings. Note TraceValues and RTraceValues have different types, but the
	// same underlying type: map[string][]string
	q.TraceValues = validate.QueryFormValue(r, "query")
	q.RTraceValues = validate.QueryFormValue(r, "rquery")

	// TODO(stephan) Add range limiting to the validation of limit and offset.
	q.Limit = int32(validate.Int64FormValue(r, "limit", 50))
	q.Offset = int32(validate.Int64FormValue(r, "offset", 0))
	q.Offset = util.MaxInt32(q.Offset, 0)

	validate.StrFormValue(r, "metric", &q.Metric, diff.GetDiffMetricIDs(), diff.CombinedMetric)
	validate.StrFormValue(r, "sort", &q.Sort, []string{SortDescending, SortAscending}, SortDescending)

	// Parse and validate the filter values.
	q.FRGBAMin = int32(validate.Int64FormValue(r, "frgbamin", 0))
	q.FRGBAMax = int32(validate.Int64FormValue(r, "frgbamax", 255))
	q.FDiffMax = float32(validate.Float64FormValue(r, "fdiffmax", -1.0))

	// Parse out the issue and patchsets.
	q.PatchSets = validate.Int64SliceFormValue(r, "patchsets", nil)
	q.ChangeListID = r.FormValue("issue")

	// Check whether any of the validations failed.
	if err := validate.Errors(); err != nil {
		return skerr.Wrapf(err, "validating params")
	}

	q.BlameGroupID = r.FormValue("blame")
	q.Pos = r.FormValue("pos") == "true"
	q.Neg = r.FormValue("neg") == "true"
	q.Unt = r.FormValue("unt") == "true"
	q.Head = r.FormValue("head") == "true"
	q.IncludeIgnores = r.FormValue("include") == "true"
	q.IncludeMaster = r.FormValue("master") == "true"

	// Extract the filter values.
	q.FCommitBegin = r.FormValue("fbegin")
	q.FCommitEnd = r.FormValue("fend")
	q.FGroupTest = r.FormValue("fgrouptest")
	q.FRef = r.FormValue("fref") == "true"

	// Check if we want diffs.
	q.NoDiff = r.FormValue("nodiff") == "true"
	q.NewCLStore = r.FormValue("new_clstore") == "true"

	return nil
}
