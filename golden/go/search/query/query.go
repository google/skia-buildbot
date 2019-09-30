// Package query contains the logic involving parsing queries to
// Gold's search endpoints.
package query

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

const (
	// SortByImageCounts indicates that the image counts should be used for sorting.
	SortByImageCounts = "count"

	// SortByDiff indicates that the diff field should be used for sorting.
	SortByDiff = "diff"

	// SortAscending indicates that we want to sort in ascending order.
	SortAscending = "asc"

	// SortDescending indicates that we want to sort in descending order.
	SortDescending = "desc"

	// maxLimit is the maximum number of digests we will return.
	maxLimit = 200
)

var (
	// sortDirections are the valid options for any of the sort direction fields.
	sortDirections = []string{SortAscending, SortDescending}

	// rowSortFields are the valid options for the sort field for rows.
	rowSortFields = []string{SortByImageCounts, SortByDiff}

	// columnSortFields are the valid options for the sort field for columns.
	columnSortFields = []string{SortByDiff}
)

// ParseDTQuery parses JSON from the given ReadCloser into the given
// pointer to an instance of DigestTable. It will fill in values and validate key
// fields of the query. It will return an error if parsing failed
// for some reason and always close the ReadCloser. testName is the name of the
// test that should be compared and limitDefault is the default limit for the
// row and column queries.
func ParseDTQuery(r io.ReadCloser, limitDefault int32, q *DigestTable) error {
	defer util.Close(r)
	var err error

	// Parse the body of the JSON request.
	if err = json.NewDecoder(r).Decode(q); err != nil {
		return skerr.Wrapf(err, "decoding JSON")
	}

	if (q.RowQuery == nil) || (q.ColumnQuery == nil) {
		return skerr.Fmt("RowQuery and ColumnQuery must not be null.")
	}

	// Parse the query string into a url.Values instance.
	if q.RowQuery.TraceValues, err = url.ParseQuery(q.RowQuery.QueryStr); err != nil {
		return skerr.Wrapf(err, "parsing RowQuery %s", q.RowQuery.QueryStr)
	}
	if q.ColumnQuery.TraceValues, err = url.ParseQuery(q.ColumnQuery.QueryStr); err != nil {
		return skerr.Wrapf(err, "parsing ColumnQuery %s", q.ColumnQuery.QueryStr)
	}

	rowCorpus := q.RowQuery.TraceValues.Get(types.CORPUS_FIELD)
	colCorpus := q.ColumnQuery.TraceValues.Get(types.CORPUS_FIELD)
	if (rowCorpus != colCorpus) || (rowCorpus == "") {
		return skerr.Fmt("Corpus for row and column query need to match and be non-empty.")
	}

	// Make sure that the name is forced to match.
	if !util.In(types.PRIMARY_KEY_FIELD, q.Match) {
		q.Match = append(q.Match, types.PRIMARY_KEY_FIELD)
	}

	// Set the limit to a default if not set.
	if q.RowQuery.Limit == 0 {
		q.RowQuery.Limit = int32(limitDefault)
	}
	q.RowQuery.Limit = util.MinInt32(q.RowQuery.Limit, maxLimit)

	if q.ColumnQuery.Limit == 0 {
		q.ColumnQuery.Limit = limitDefault
	}
	q.ColumnQuery.Limit = util.MinInt32(q.ColumnQuery.Limit, maxLimit)

	validate := shared.Validation{}

	// Parse the patchsets.
	q.ColumnQuery.PatchSets = validate.Int64SliceValue("patchsets", q.ColumnQuery.PatchSetsStr, nil)
	q.RowQuery.PatchSets = validate.Int64SliceValue("patchsets", q.RowQuery.PatchSetsStr, nil)

	// Parse the general parameters of the query.
	validate.StrValue("sortRows", &q.SortRows, rowSortFields, SortByImageCounts)
	validate.StrValue("rowsDir", &q.RowsDir, sortDirections, SortDescending)
	validate.StrValue("sortColumns", &q.SortColumns, columnSortFields, SortByDiff)
	validate.StrValue("columnsDir", &q.ColumnsDir, sortDirections, SortAscending)
	validate.StrValue("metrics", &q.Metric, diff.GetDiffMetricIDs(), diff.METRIC_PERCENT)
	return validate.Errors()
}

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
		if !util.In(types.PRIMARY_KEY_FIELD, q.Match) {
			q.Match = append(q.Match, types.PRIMARY_KEY_FIELD)
		}
	} else {
		q.Match = []string{types.PRIMARY_KEY_FIELD}
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

	validate.StrFormValue(r, "metric", &q.Metric, diff.GetDiffMetricIDs(), diff.METRIC_COMBINED)
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
