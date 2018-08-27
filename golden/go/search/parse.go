package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

const (
	// SORT_FIELD_COUNT indicates that the image counts should be used for sorting.
	SORT_FIELD_COUNT = "count"

	// SORT_FIELD_DIFF indicates that the diff field should be used for sorting.
	SORT_FIELD_DIFF = "diff"
)

var (
	// sortDirections are the valid options for any of the sort direction fields.
	sortDirections = []string{SORT_ASC, SORT_DESC}

	// rowSortFields are the valid options for the sort field for rows.
	rowSortFields = []string{SORT_FIELD_COUNT, SORT_FIELD_DIFF}

	// columnSortFields are the valid options for the sort field for columns.
	columnSortFields = []string{SORT_FIELD_DIFF}
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
	ctQuery.RowQuery.Limit = util.MinInt32(ctQuery.RowQuery.Limit, MAX_LIMIT)

	if ctQuery.ColumnQuery.Limit == 0 {
		ctQuery.ColumnQuery.Limit = limitDefault
	}
	ctQuery.ColumnQuery.Limit = util.MinInt32(ctQuery.ColumnQuery.Limit, MAX_LIMIT)

	validate := Validation{}

	// Parse the patchsets.
	ctQuery.ColumnQuery.Patchsets = validate.Int64SliceValue("patchsets", ctQuery.ColumnQuery.PatchsetsStr, nil)
	ctQuery.RowQuery.Patchsets = validate.Int64SliceValue("patchsets", ctQuery.RowQuery.PatchsetsStr, nil)

	ctQuery.ColumnQuery.Issue = validate.Int64Value("column.issue", ctQuery.ColumnQuery.IssueStr, 0)
	ctQuery.RowQuery.Issue = validate.Int64Value("row.issue", ctQuery.RowQuery.IssueStr, 0)

	// Parse the general parameters of the query.
	validate.StrValue("sortRows", &ctQuery.SortRows, rowSortFields, SORT_FIELD_COUNT)
	validate.StrValue("rowsDir", &ctQuery.RowsDir, sortDirections, SORT_DESC)
	validate.StrValue("sortColumns", &ctQuery.SortColumns, columnSortFields, SORT_FIELD_DIFF)
	validate.StrValue("columnsDir", &ctQuery.ColumnsDir, sortDirections, SORT_ASC)
	validate.StrValue("metrics", &ctQuery.Metric, diff.GetDiffMetricIDs(), diff.METRIC_PERCENT)
	return validate.Errors()
}

// TODO(stephana): Validation should be factored out into a separate package.

// Validation is a container to collect error messages during validation of a
// input with multiple fields.
type Validation []string

// StrValue validates a string value against containment in a set of options.
// Argument:
//     name: name of the field being validated.
//     val: value to be validated.
//     options: list of options, one of which value can contain.
//     defaultVal: default value in case val is empty. Can be equal to "".
// If there is a problem an error message will be added to the Validation object.
func (v *Validation) StrValue(name string, val *string, options []string, defaultVal string) {
	if *val == "" && defaultVal != "" {
		*val = defaultVal
		return
	}
	if !util.In(*val, options) {
		*v = append(*v, fmt.Sprintf("Field '%s' needs to be one of '%s'", name, strings.Join(options, ",")))
	}
}

// StrFormValue does the same as StrValue but extracts the given name from
// the request via r.FormValue(..).
func (v *Validation) StrFormValue(r *http.Request, name string, val *string, options []string, defaultVal string) {
	*val = r.FormValue(name)
	v.StrValue(name, val, options, defaultVal)
}

// Float64Value parses the value given in strVal and returns it. If strVal is empty
// the default value is returned.
func (v *Validation) Float64Value(name string, strVal string, defaultVal float64) float64 {
	if strVal == "" {
		return defaultVal
	}

	tempVal, err := strconv.ParseFloat(strVal, 64)
	if err != nil {
		*v = append(*v, fmt.Sprintf("Field '%s' is not a valid float: %s", name, err))
	}
	return tempVal
}

// Int64Value parses the value given in strVal and returns it. If strVal is empty
// the default value is returned.
func (v *Validation) Int64Value(name string, strVal string, defaultVal int64) int64 {
	if strVal == "" {
		return defaultVal
	}

	tempVal, err := strconv.ParseInt(strVal, 10, 64)
	if err != nil {
		*v = append(*v, fmt.Sprintf("Field '%s' is not a valid int: %s", name, err))
	}
	return tempVal
}

// Float64FormValue does the same as Float64Value but extracts the value from the request object.
func (v *Validation) Float64FormValue(r *http.Request, name string, defaultVal float64) float64 {
	return v.Float64Value(name, r.FormValue(name), defaultVal)
}

// Int64FormValue does the same as Int64Value but extracts the value from the request object.
func (v *Validation) Int64FormValue(r *http.Request, name string, defaultVal int64) int64 {
	return v.Int64Value(name, r.FormValue(name), defaultVal)
}

// Int64SliceValue parses a comma-separated list of int values and returns them.
func (v *Validation) Int64SliceValue(name string, strVal string, defaultVal []int64) []int64 {
	if strVal == "" {
		return defaultVal
	}

	splitVals := strings.Split(strVal, ",")
	ret := make([]int64, 0, len(splitVals))
	for _, oneStrVal := range splitVals {
		tempVal, err := strconv.ParseInt(oneStrVal, 10, 64)
		if err != nil {
			*v = append(*v, fmt.Sprintf("Field '%s' is not a valid list of comma separated integers: %s", name, err))
			return nil
		}
		ret = append(ret, tempVal)
	}
	return ret
}

// Int64SliceFormValue does the same as Int64SliceValue but extracts the given
// name from the request.
func (v *Validation) Int64SliceFormValue(r *http.Request, name string, defaultVal []int64) []int64 {
	return v.Int64SliceValue(name, r.FormValue(name), defaultVal)
}

// QueryFormValue extracts a URL-encoded query from the form values and decodes it.
// If the named field was not available in the given request an empty set url.Values
// is returned. If an error occurs it will be added to the error list of the validation
// object.
func (v *Validation) QueryFormValue(r *http.Request, name string) map[string][]string {
	if q := r.FormValue(name); q != "" {
		ret, err := url.ParseQuery(q)
		if err != nil {
			*v = append(*v, fmt.Sprintf("Unable to parse query: %s. Error: %s", q, err))
			return nil
		}
		return ret
	}
	return map[string][]string{}
}

// Errors returns a concatenation of all error values accumulated in validation or nil
// if there were no errors.
func (v *Validation) Errors() error {
	if len(*v) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(*v, "\n"))
}

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

	validate := Validation{}

	// Parse the query strings. Note Query and RQuery have different types, but the
	// same underlying type: map[string][]string
	query.Query = validate.QueryFormValue(r, "query")
	query.RQuery = validate.QueryFormValue(r, "rquery")

	// TODO(stephan) Add range limiting to the validation of limit and offset.
	query.Limit = int32(validate.Int64FormValue(r, "limit", 50))
	query.Offset = int32(validate.Int64FormValue(r, "offset", 0))
	query.Offset = util.MaxInt32(query.Offset, 0)

	validate.StrFormValue(r, "metric", &query.Metric, diff.GetDiffMetricIDs(), diff.METRIC_COMBINED)
	validate.StrFormValue(r, "sort", &query.Sort, []string{SORT_DESC, SORT_ASC}, SORT_DESC)

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
