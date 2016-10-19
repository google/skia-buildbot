package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// ParseCTQuery parses JSON from the given ReadCloser into the given
// pointer to an instance of CTQuery. It will fill in values and validate key
// fields of the query. It will return an error if parsing failed
// for some reason and always close the ReadCloser. testName is the name of the
// test that should be compared and limitDefault is the default limit for the
// row and column queries.
func ParseCTQuery(r io.ReadCloser, testName string, limitDefault int, ctQuery *CTQuery) error {
	defer util.Close(r)
	var err error

	inputJson, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	r = ioutil.NopCloser(bytes.NewBuffer(inputJson))
	glog.Infof("QUERY: \n\n%s\n\n", string(inputJson))

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

	// Make sure the test is set right.
	if testName != "" {
		ctQuery.ColumnQuery.Query.Set(types.PRIMARY_KEY_FIELD, testName)
		ctQuery.RowQuery.Query.Set(types.PRIMARY_KEY_FIELD, testName)
	}

	// Set the limit to a default if not set.
	if ctQuery.RowQuery.Limit == 0 {
		ctQuery.RowQuery.Limit = limitDefault
	}
	if ctQuery.ColumnQuery.Limit == 0 {
		ctQuery.ColumnQuery.Limit = limitDefault
	}
	return nil
}
