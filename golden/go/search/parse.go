package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// ParseCTQuery parses JSON from the given ReadCloser into the given
// pointer to an instance of CTQuery. It will return an error if parsing failed
// for some reason and always close the ReadCloser.
func ParseCTQuery(r io.ReadCloser, ctQuery *CTQuery) error {
	defer util.Close(r)

	// Parse the body of the JSON request.
	if err := json.NewDecoder(r).Decode(ctQuery); err != nil {
		return err
	}

	// Parse the query string into a url.Values instance.
	var err error
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

	if ctQuery.Test == "" {
		return fmt.Errorf("Test in compare query cannot be empty.")
	}

	// Make sure the test is set right.
	ctQuery.ColumnQuery.Query.Set(types.PRIMARY_KEY_FIELD, ctQuery.Test)
	ctQuery.RowQuery.Query.Set(types.PRIMARY_KEY_FIELD, ctQuery.Test)

	// Set the limit to a default if not set.

	return nil
}
