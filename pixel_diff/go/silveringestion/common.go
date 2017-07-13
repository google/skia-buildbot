package silveringestion
// The JSON output from CT looks like this:
// {
//  “run_id” : “{userid-timestamp}”,
//  “patch” : “{link to patch}”,
//  “screenshots” : [
//    {
//      “type” : “{nopatch/withpatch}”,
//      “filename” : “{GS filename}”,
//    }, ...
//  ]
// }

import (
	"encoding/json"
	"fmt"
	"io"

	"go.skia.org/infra/go/util"
)

// Screenshot contains the information for a screenshot taken by CT.
type Screenshot struct {
	Type     string   `json:"type"`
	Filename string   `json:"filename"`
}

// CTResults is the top level structure for decoding CT JSON output.
type CTResults struct {
	RunID         string          `json:"run_id"`
	Patch         string          `json:"patch"`
	Screenshots   []*Screenshot   `json:"screenshots"`

	// name is the name/path of the file where the data came from.
	name          string
}

func ParseCTResultsFromReader(r io.ReadCloser, name string) (*CTResults, error) {
	defer util.Close(r)

	dec := json.NewDecoder(r)
	ctResults := &CTResults{}
	if err := dec.Decode(ctResults); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	ctResults.name = name
	return ctResults, nil
}
