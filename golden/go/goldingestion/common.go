package goldingestion

// The JSON output from DM looks like this:
//
//  {
//     "build_number" : "20",
//     "gitHash" : "abcd",
//     "key" : {
//        "arch" : "x86",
//        "configuration" : "Debug",
//        "gpu" : "nvidia",
//        "model" : "z620",
//        "os" : "Ubuntu13.10"
//     },
//     "results" : [
//        {
//           "key" : {
//              "config" : "565",
//              "name" : "ninepatch-stretch",
//              "source_type" : "gm"
//           },
//           "md5" : "f78cfafcbabaf815f3dfcf61fb59acc7",
//           "options" : {
//              "ext" : "png"
//           }
//        },
//        {
//           "key" : {
//              "config" : "8888",
//              "name" : "ninepatch-stretch",
//              "source_type" : "gm"
//           },
//           "md5" : "3e8a42f35a1e76f00caa191e6310d789",
//           "options" : {
//              "ext" : "png"
//           }

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
)

// Result is used by DMResults hand holds the invidual result of one test.
type Result struct {
	Key     map[string]string `json:"key"`
	Options map[string]string `json:"options"`
	Digest  string            `json:"md5"`
}

// DMResults is the top level structure for decoding DM JSON output.
type DMResults struct {
	Master      string            `json:"master"`
	Builder     string            `json:"builder"`
	BuildNumber string            `json:"build_number"`
	GitHash     string            `json:"gitHash"`
	Key         map[string]string `json:"key"`
	Issue       int64             `json:"issue,string"`
	Patchset    int64             `json:"patchset,string"`
	Results     []*Result         `json:"results"`
}

// idAndParams constructs the Trace ID and the Trace params from the keys and options.
func (d *DMResults) idAndParams(r *Result) (string, map[string]string) {
	combinedLen := len(d.Key) + len(r.Key)
	traceIdParts := make(map[string]string, combinedLen)
	params := make(map[string]string, combinedLen+1)

	// Add the builder field to params.
	params["builder"] = d.Builder

	for k, v := range d.Key {
		traceIdParts[k] = v
		params[k] = v
	}
	for k, v := range r.Key {
		traceIdParts[k] = v
		params[k] = v
	}
	for k, v := range r.Options {
		params[k] = v
	}

	keys := []string{}
	for k, _ := range traceIdParts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	values := []string{}
	for _, k := range keys {
		values = append(values, traceIdParts[k])
	}
	return strings.Join(values, ":"), params
}

// getTraceDBEntries returns the traceDB entries to be inserted into the data store.
func (d *DMResults) getTraceDBEntries() map[string]*tracedb.Entry {
	ret := make(map[string]*tracedb.Entry, len(d.Results))
	for _, result := range d.Results {
		traceId, params := d.idAndParams(result)
		if d.ignoreResult(params) {
			continue
		}

		ret[traceId] = &tracedb.Entry{
			Params: params,
			Value:  []byte(result.Digest),
		}
	}
	return ret
}

// ignoreResult returns true if the result with the given parameters should be
// ignored.
func (d *DMResults) ignoreResult(params map[string]string) bool {
	// Ignore anything that is not a png.
	ext, ok := params["ext"]
	return !ok || (ext != "png")
}

// ParseDMResultsFromReader parses the stream out of the io.ReadCloser
// into a DMResults instance and closes the reader.
func ParseDMResultsFromReader(r io.ReadCloser) (*DMResults, error) {
	defer util.Close(r)

	dec := json.NewDecoder(r)
	dmResults := &DMResults{}
	if err := dec.Decode(dmResults); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return dmResults, nil
}

// FilterCommitIDs returns all commitIDs that have the given prefix. If the
// prefix is an empty string it will return the input slice.
func FilterCommitIDs(commitIDs []*tracedb.CommitID, prefix string) []*tracedb.CommitID {
	if prefix == "" {
		return commitIDs
	}

	ret := make([]*tracedb.CommitID, 0, len(commitIDs))
	for _, cid := range commitIDs {
		if strings.HasPrefix(cid.Source, prefix) {
			ret = append(ret, cid)
		}
	}
	return ret
}
