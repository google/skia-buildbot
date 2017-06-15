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

	// Parse the list of patchsets from a comma separated list in the stringsted.
	ctQuery.ColumnQuery.Patchsets = strings.Split(ctQuery.ColumnQuery.PatchsetsStr, ",")
	ctQuery.RowQuery.Patchsets = strings.Split(ctQuery.RowQuery.PatchsetsStr, ",")

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

// Float32Value parses the value given in strVal and stores it in *val. If strVal is empty
// the default value is written to *val.
func (v *Validation) Float32Value(name string, strVal string, val *float32, defaultVal float32) {
	if strVal == "" {
		*val = defaultVal
		return
	}

	tempVal, err := strconv.ParseFloat(strVal, 32)
	if err != nil {
		*v = append(*v, fmt.Sprintf("Field '%s' is not a valid float: %s", name, err))
	}
	*val = float32(tempVal)
}

// Int32Value parses the value given in strVal and stores it in *val. If strVal is empty
// the default value is written to *val.
func (v *Validation) Int32Value(name string, strVal string, val *int32, defaultVal int32) {
	if strVal == "" {
		*val = defaultVal
		return
	}

	tempVal, err := strconv.ParseInt(strVal, 10, 32)
	if err != nil {
		*v = append(*v, fmt.Sprintf("Field '%s' is not a valid int: %s", name, err))
	}
	*val = int32(tempVal)
}

// Float32FormValue does the same as Float32Value but extracts the value from the request object.
func (v *Validation) Float32FormValue(r *http.Request, name string, val *float32, defaultVal float32) {
	v.Float32Value(name, r.FormValue(name), val, defaultVal)
}

// Int32FormValue does the same as Int32Value but extracts the value from the request object.
func (v *Validation) Int32FormValue(r *http.Request, name string, val *int32, defaultVal int32) {
	v.Int32Value(name, r.FormValue(name), val, defaultVal)
}

// QueryFormValue extracts a URL-encoded query from the form values and decodes it.
// If the named field was not available in the given request an empty set url.Values
// is returned. If an error occurs it will be added to the error list of the validation
// object.
func (v *Validation) QueryFormValue(r *http.Request, name string, val *url.Values) {
	if q := r.FormValue(name); q != "" {
		var err error
		*val, err = url.ParseQuery(q)
		if err != nil {
			*v = append(*v, fmt.Sprintf("Unable to parse query: %s. Error: %s", q, err))
		}
	} else {
		*val = url.Values{}
	}
}

// Errors returns a concatination of all error values accumulated in validation or nil
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
	// Parse out the patchsets.
	if temp := r.FormValue("patchsets"); temp != "" {
		query.Patchsets = strings.Split(temp, ",")
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

	// Parse the query strings.
	validate.QueryFormValue(r, "query", &query.Query)
	validate.QueryFormValue(r, "rquery", &query.RQuery)

	// TODO(stephan) Add range limiting to the validation of limit and offset.
	validate.Int32FormValue(r, "limit", &query.Limit, 50)
	validate.Int32FormValue(r, "offset", &query.Offset, 0)
	query.Offset = util.MaxInt32(query.Offset, 0)

	validate.StrFormValue(r, "metric", &query.Metric, diff.GetDiffMetricIDs(), diff.METRIC_COMBINED)
	validate.StrFormValue(r, "sort", &query.Sort, []string{SORT_DESC, SORT_ASC}, SORT_DESC)

	// Parse and validate the filter values.
	validate.Int32FormValue(r, "frgbamin", &query.FRGBAMin, 0)
	validate.Int32FormValue(r, "frgbamax", &query.FRGBAMax, 255)
	validate.Float32FormValue(r, "fdiffmax", &query.FDiffMax, -1.0)
	if err := validate.Errors(); err != nil {
		return err
	}

	query.BlameGroupID = r.FormValue("blame")
	query.Pos = r.FormValue("pos") == "true"
	query.Neg = r.FormValue("neg") == "true"
	query.Unt = r.FormValue("unt") == "true"
	query.Head = r.FormValue("head") == "true"
	query.IncludeIgnores = r.FormValue("include") == "true"
	query.Issue = r.FormValue("issue")
	query.IncludeMaster = r.FormValue("master") == "true"

	// Extract the filter values.
	query.FCommitBegin = r.FormValue("fbegin")
	query.FCommitEnd = r.FormValue("fend")
	query.FGroupTest = r.FormValue("fgrouptest")
	query.FRef = r.FormValue("fref") == "true"

	return nil
}
