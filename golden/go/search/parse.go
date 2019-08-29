package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/types"
)

const (
	// countSortField indicates that the image counts should be used for sorting.
	countSortField = "count"

	// diffSortField indicates that the diff field should be used for sorting.
	diffSortField = "diff"
)

var (
	// sortDirections are the valid options for any of the sort direction fields.
	sortDirections = []string{sortAscending, sortDescending}

	// rowSortFields are the valid options for the sort field for rows.
	rowSortFields = []string{countSortField, diffSortField}

	// columnSortFields are the valid options for the sort field for columns.
	columnSortFields = []string{diffSortField}
)

// ParseCTQuery parses JSON from the given ReadCloser into the given
// pointer to an instance of CTQuery. It will fill in values and validate key
// fields of the query. It will return an error if parsing failed
// for some reason and always close the ReadCloser. testName is the name of the
// test that should be compared and limitDefault is the default limit for the
// row and column queries.
func ParseCTQuery(r io.ReadCloser, limitDefault int32, ctQuery *CTQuery) error {
	defer util.Close(r)
	var err error

	// Parse the body of the JSON request.
	if err := json.NewDecoder(r).Decode(ctQuery); err != nil {
		return err
	}

	if (ctQuery.RowQuery == nil) || (ctQuery.ColumnQuery == nil) {
		return fmt.Errorf("Rowquery and columnquery must not be null.")
	}

	// Parse the query string into a url.Values instance.
	if ctQuery.RowQuery.Query, err = url.ParseQuery(ctQuery.RowQuery.QueryStr); err != nil {
		return err
	}
	if ctQuery.ColumnQuery.Query, err = url.ParseQuery(ctQuery.ColumnQuery.QueryStr); err != nil {
		return err
	}

	rowCorpus := ctQuery.RowQuery.Query.Get(types.CORPUS_FIELD)
	colCorpus := ctQuery.ColumnQuery.Query.Get(types.CORPUS_FIELD)
	if (rowCorpus != colCorpus) || (rowCorpus == "") {
		return fmt.Errorf("Corpus for row and column query need to match and be non-empty.")
	}

	// Make sure that the name is forced to match.
	if !util.In(types.PRIMARY_KEY_FIELD, ctQuery.Match) {
		ctQuery.Match = append(ctQuery.Match, types.PRIMARY_KEY_FIELD)
	}

	// TODO(stephana): Factor out setting default values and limiting values.
	// Also factor this out together with the Validation type.

	// Set the limit to a default if not set.
	if ctQuery.RowQuery.Limit == 0 {
		ctQuery.RowQuery.Limit = int32(limitDefault)
	}
	ctQuery.RowQuery.Limit = util.MinInt32(ctQuery.RowQuery.Limit, maxLimit)

	if ctQuery.ColumnQuery.Limit == 0 {
		ctQuery.ColumnQuery.Limit = limitDefault
	}
	ctQuery.ColumnQuery.Limit = util.MinInt32(ctQuery.ColumnQuery.Limit, maxLimit)

	validate := shared.Validation{}

	// Parse the patchsets.
	ctQuery.ColumnQuery.Patchsets = validate.Int64SliceValue("patchsets", ctQuery.ColumnQuery.PatchsetsStr, nil)
	ctQuery.RowQuery.Patchsets = validate.Int64SliceValue("patchsets", ctQuery.RowQuery.PatchsetsStr, nil)

	ctQuery.ColumnQuery.Issue = validate.Int64Value("column.issue", ctQuery.ColumnQuery.IssueStr, 0)
	ctQuery.RowQuery.Issue = validate.Int64Value("row.issue", ctQuery.RowQuery.IssueStr, 0)

	// Parse the general parameters of the query.
	validate.StrValue("sortRows", &ctQuery.SortRows, rowSortFields, countSortField)
	validate.StrValue("rowsDir", &ctQuery.RowsDir, sortDirections, sortDescending)
	validate.StrValue("sortColumns", &ctQuery.SortColumns, columnSortFields, diffSortField)
	validate.StrValue("columnsDir", &ctQuery.ColumnsDir, sortDirections, sortAscending)
	validate.StrValue("metrics", &ctQuery.Metric, diff.GetDiffMetricIDs(), diff.METRIC_PERCENT)
	return validate.Errors()
}

// TODO(stephana): Validation should be factored out into a separate package.

// ParseQuery parses the request parameters from the URL query string or from the
// form parameters and stores the parsed and validated values in query.
func ParseQuery(r *http.Request, query *Query) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	// Parse the list of fields that need to match and ensure the
	// test name is in it.
	var ok bool
	if query.Match, ok = r.Form["match"]; ok {
		if !util.In(types.PRIMARY_KEY_FIELD, query.Match) {
			query.Match = append(query.Match, types.PRIMARY_KEY_FIELD)
		}
	} else {
		query.Match = []string{types.PRIMARY_KEY_FIELD}
	}

	validate := shared.Validation{}

	// Parse the query strings. Note Query and RQuery have different types, but the
	// same underlying type: map[string][]string
	query.Query = validate.QueryFormValue(r, "query")
	query.RQuery = validate.QueryFormValue(r, "rquery")

	// TODO(stephan) Add range limiting to the validation of limit and offset.
	query.Limit = int32(validate.Int64FormValue(r, "limit", 50))
	query.Offset = int32(validate.Int64FormValue(r, "offset", 0))
	query.Offset = util.MaxInt32(query.Offset, 0)

	validate.StrFormValue(r, "metric", &query.Metric, diff.GetDiffMetricIDs(), diff.METRIC_COMBINED)
	validate.StrFormValue(r, "sort", &query.Sort, []string{sortDescending, sortAscending}, sortDescending)

	// Parse and validate the filter values.
	query.FRGBAMin = int32(validate.Int64FormValue(r, "frgbamin", 0))
	query.FRGBAMax = int32(validate.Int64FormValue(r, "frgbamax", 255))
	query.FDiffMax = float32(validate.Float64FormValue(r, "fdiffmax", -1.0))

	// Parse out the issue and patchsets.
	query.Patchsets = validate.Int64SliceFormValue(r, "patchsets", nil)
	query.Issue = validate.Int64FormValue(r, "issue", 0)

	// Check wether any of the validations failed.
	if err := validate.Errors(); err != nil {
		return err
	}

	query.BlameGroupID = r.FormValue("blame")
	query.Pos = r.FormValue("pos") == "true"
	query.Neg = r.FormValue("neg") == "true"
	query.Unt = r.FormValue("unt") == "true"
	query.Head = r.FormValue("head") == "true"
	query.IncludeIgnores = r.FormValue("include") == "true"
	query.IncludeMaster = r.FormValue("master") == "true"

	// Extract the filter values.
	query.FCommitBegin = r.FormValue("fbegin")
	query.FCommitEnd = r.FormValue("fend")
	query.FGroupTest = r.FormValue("fgrouptest")
	query.FRef = r.FormValue("fref") == "true"

	// Check if we want diffs.
	query.NoDiff = r.FormValue("nodiff") == "true"

	return nil
}
