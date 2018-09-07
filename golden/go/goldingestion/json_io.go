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
)

// GoldResults is the top level structure to capture the the results of a
// rendered test to be processed by Gold.
type GoldResults struct {
	BuildBucketID int64             `json:"buildbucket_build_id,string"`
	GitHash       string            `json:"gitHash"`
	Key           map[string]string `json:"key"`
	Issue         int64             `json:"issue,string"`
	Patchset      int64             `json:"patchset,string"`
	Results       []*Result         `json:"results"`

	// ??? Not needed ???
	// SwarmingTaskID string            `json:"swarming_task_id"`
	// SwarmingBotID  string            `json:"swarming_bot_id"`
	// PatchStorage   string            `json:"patch_storage"`
	// 	Builder string `json:"builder"`
}

// Result is used by DMResults hand holds the individual result of one test.
type Result struct {
	Key     map[string]string `json:"key"`
	Options map[string]string `json:"options"`
	Digest  string            `json:"md5"`
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

// toStringMap converts the given generic map to map[string]string.
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
