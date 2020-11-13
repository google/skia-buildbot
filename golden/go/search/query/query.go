// Package query contains the logic involving parsing queries to
// Gold's search endpoints.
package query

import (
	"net/http"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

const (
	// SortAscending indicates that we want to sort in ascending order.
	SortAscending = "asc"
	// SortDescending indicates that we want to sort in descending order.
	SortDescending = "desc"

	// CombinedMetric corresponds to diff.DiffMetric.CombinedMetric
	CombinedMetric = "combined"
	// PercentMetric corresponds to diff.DiffMetric.PixelDiffPercent
	PercentMetric = "percent"
	// PixelMetric corresponds to diff.DiffMetric.NumDiffPixels
	PixelMetric = "pixel"
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

	// Parse the query strings.
	q.TraceValues = validate.QueryFormValue(r, "query")
	q.RightTraceValues = validate.QueryFormValue(r, "rquery")

	q.Limit = int32(validate.Int64FormValue(r, "limit", 50))
	q.Offset = int32(validate.Int64FormValue(r, "offset", 0))
	q.Offset = util.MaxInt32(q.Offset, 0)

	validate.StrFormValue(r, "metric", &q.Metric, []string{CombinedMetric, PercentMetric, PixelMetric}, CombinedMetric)
	validate.StrFormValue(r, "sort", &q.Sort, []string{SortDescending, SortAscending}, SortDescending)

	// Parse and validate the filter values.
	q.RGBAMinFilter = int32(validate.Int64FormValue(r, "frgbamin", 0))
	q.RGBAMaxFilter = int32(validate.Int64FormValue(r, "frgbamax", 255))
	q.DiffMaxFilter = float32(validate.Float64FormValue(r, "fdiffmax", -1.0))

	// Parse out the issue and patchsets.
	q.PatchSets = validate.Int64SliceFormValue(r, "patchsets", nil)
	q.ChangeListID = r.FormValue("issue")

	// Check whether any of the validations failed.
	if err := validate.Errors(); err != nil {
		return skerr.Wrapf(err, "validating params")
	}

	q.BlameGroupID = r.FormValue("blame")
	q.IncludePositiveDigests = r.FormValue("pos") == "true"
	q.IncludeNegativeDigests = r.FormValue("neg") == "true"
	q.IncludeUntriagedDigests = r.FormValue("unt") == "true"
	q.OnlyIncludeDigestsProducedAtHead = r.FormValue("head") == "true"
	q.IncludeIgnoredTraces = r.FormValue("include") == "true"
	q.IncludeDigestsProducedOnMaster = r.FormValue(git.DefaultBranch) == "true"

	// Extract the filter values.
	q.CommitBeginFilter = r.FormValue("fbegin")
	q.CommitEndFilter = r.FormValue("fend")
	q.GroupTestFilter = r.FormValue("fgrouptest")
	q.MustIncludeReferenceFilter = r.FormValue("fref") == "true"

	// Check if we want diffs.
	q.NoDiff = r.FormValue("nodiff") == "true"

	return nil
}
