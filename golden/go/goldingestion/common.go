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
//

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sklog"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

// Result is used by DMResults hand holds the individual result of one test.
type Result struct {
	Key     map[string]string `json:"key"`
	Options map[string]string `json:"options"`
	Digest  string            `json:"md5"`
}

func toStringMap(interfaceMap map[string]interface{}) (map[string]string, error) {
	ret := make(map[string]string, len(interfaceMap))
	for k, v := range interfaceMap {
		switch val := v.(type) {
		case bool:
			if val {
				ret[k] = "yes"
			} else {
				ret[k] = "no"
			}
		case string:
			ret[k] = val
		default:
			return nil, fmt.Errorf("Unable to convert %#v to string map.", interfaceMap)
		}
	}

	return ret, nil
}

// TODO(stephana) Potentially remove this function once gamma_corrected field contains
// only strings.
func (r *Result) UnmarshalJSON(data []byte) error {
	var err error
	container := map[string]interface{}{}
	if err := json.Unmarshal(data, &container); err != nil {
		return err
	}

	key, ok := container["key"]
	if !ok {
		return fmt.Errorf("Did not get key field in result.")
	}

	options, ok := container["options"]
	if !ok {
		return fmt.Errorf("Did not get options field in result.")
	}

	digest, ok := container["md5"].(string)
	if !ok {
		return fmt.Errorf("Did not get md5 field in result.")
	}

	if r.Key, err = toStringMap(key.(map[string]interface{})); err != nil {
		return err
	}

	if r.Options, err = toStringMap(options.(map[string]interface{})); err != nil {
		return err
	}
	r.Digest = digest
	return nil
}

// DMResults is the top level structure for decoding DM JSON output.
type DMResults struct {
	Builder        string            `json:"builder"`
	GitHash        string            `json:"gitHash"`
	Key            map[string]string `json:"key"`
	Issue          int64             `json:"issue,string"`
	Patchset       int64             `json:"patchset,string"`
	Results        []*Result         `json:"results"`
	PatchStorage   string            `json:"patch_storage"`
	SwarmingBotID  string            `json:"swarming_bot_id"`
	SwarmingTaskID string            `json:"swarming_task_id"`
	BuildBucketID  int64             `json:"buildbucket_build_id,string"`

	// name is the name/path of the file where this came from.
	name string
}

// idAndParams constructs the Trace ID and the Trace params from the keys and options.
func (d *DMResults) idAndParams(r *Result) (string, map[string]string) {
	combinedLen := len(d.Key) + len(r.Key)
	traceIdParts := make(map[string]string, combinedLen)
	params := make(map[string]string, combinedLen+1)
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
	for k := range traceIdParts {
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
func (d *DMResults) getTraceDBEntries() (map[string]*tracedb.Entry, error) {
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

	// If all results were ignored then we return an error.
	if len(ret) == 0 {
		return nil, fmt.Errorf("No valid results in file %s.", d.name)
	}

	return ret, nil
}

// ignoreResult returns true if the result with the given parameters should be
// ignored.
func (d *DMResults) ignoreResult(params map[string]string) bool {
	// Ignore anything that is not a png.
	if ext, ok := params["ext"]; !ok || (ext != "png") {
		return true
	}

	// Make sure the test name meets basic requirements.
	testName := params[types.PRIMARY_KEY_FIELD]

	// Ignore results that don't have a test given and log an error since that
	// should not happen. But we want to keep other results in the same input file.
	if testName == "" {
		sklog.Errorf("Missing test name in %s", d.name)
		return true
	}

	// Make sure the test name does not exceed the allowed length.
	if len(testName) > types.MAXIMUM_NAME_LENGTH {
		sklog.Errorf("Received test name which is longer than the allowed %d bytes: %s", types.MAXIMUM_NAME_LENGTH, testName)
		return true
	}

	return false
}

// Name returns the name/path from which these results were parsed.
func (d *DMResults) Name() string {
	return d.name
}

// ParseDMResultsFromReader parses the stream out of the io.ReadCloser
// into a DMResults instance and closes the reader.
func ParseDMResultsFromReader(r io.ReadCloser, name string) (*DMResults, error) {
	defer util.Close(r)

	dec := json.NewDecoder(r)
	dmResults := &DMResults{}
	if err := dec.Decode(dmResults); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	dmResults.name = name
	return dmResults, nil
}

// processDMResults opens the given input file and processes it.
func processDMResults(resultsFile ingestion.ResultFileLocation) (*DMResults, error) {
	r, err := resultsFile.Open()
	if err != nil {
		return nil, err
	}

	return ParseDMResultsFromReader(r, resultsFile.Name())
}
