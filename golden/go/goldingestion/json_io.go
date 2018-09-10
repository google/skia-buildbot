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
	"reflect"
	"strconv"
	"strings"

	"go.skia.org/infra/go/sklog"
	validator "gopkg.in/go-playground/validator.v9"
)

func ParseGoldResults(r io.Reader) (*GoldResults, map[string]string, error) {
	raw := &rawGoldResults{}
	if err := json.NewDecoder(r).Decode(raw); err != nil {
		return nil, nil, err
	}

	var fieldErrs validator.ValidationErrors = nil
	if fErrs := raw.parseValidate(); fieldErrs != nil {
		fieldErrs = append(fieldErrs, fErrs...)
	}

	ret := raw.GoldResults
	if fieldMap, err := ret.Validate(); err != nil {
		return nil, fieldMap, err
	}
	return &ret, nil, nil
}

func fieldErrorsToStringMap(err error) map[string]string {
	if fieldErrs, ok := err.(validator.ValidationErrors); ok && len(fieldErrs) > 0 {
		ret := make(map[string]string, len(fieldErrs))
		for _, fe := range fieldErrs {
			ret[fe.Field()] = fmt.Sprintf("Field '%s' is invalid. Reason: '%s'", fe.Field(), fe.Tag())
			sklog.Infof("%v %v %v %v %v", fe.Field(), fe.ActualTag(), fe.StructField(), fe.Namespace())
		}
		return ret
	}
	return nil
}

var validate *validator.Validate

func init() {
	// Initialize the validate instance used to validate objects below.
	validate = validator.New()

	// Make the returned tag be the json name.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// GoldResults is the top level structure to capture the the results of a
// rendered test to be processed by Gold.
type GoldResults struct {
	GitHash string            `json:"gitHash"  validate:"required"`
	Key     map[string]string `json:"key"      validate:"required"`
	Results []*Result         `json:"results"  validate:"min=1"`

	// Required fields for tryjobs.
	Issue         int64 `json:"issue,string"`
	BuildBucketID int64 `json:"buildbucket_build_id,string"`
	Patchset      int64 `json:"patchset,string"`

	// Optional fields
	SwarmingTaskID string `json:"swarming_task_id"`
	SwarmingBotID  string `json:"swarming_bot_id"`
	Builder        string `json:"builder"`
}

type rawGoldResults struct {
	GoldResults

	// Override the fields that represent integers as strings.
	Issue         string `json:"issue"`
	BuildBucketID string `json:"buildbucket_build_id"`
	Patchset      string `json:"patchset"`
}

func (r *rawGoldResults) parseValidate() validator.ValidationErrors {
	ret := validator.ValidationErrors(nil)
	if r.Issue == "" && (r.BuildBucketID != "" || r.Patchset != "") {
		ret = append(ret, nil)
	}

	var err error
	if r.GoldResults.Issue, err = strconv.ParseInt(r.Issue, 10, 64); err != nil {
		// append error
	}

	if r.GoldResults.BuildBucketID, err = strconv.ParseInt(r.BuildBucketID, 10, 64); err != nil {
		// append error
	}

	if r.GoldResults.Patchset, err = strconv.ParseInt(r.Patchset, 10, 64); err != nil {
		// append error
	}

	return ret
}

func (*rawGoldResults) UnmarshalJSON(data []byte) error {
	return nil
}

func (*rawGoldResults) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (g *GoldResults) Validate() (map[string]string, error) {
	var validationErrors validator.ValidationErrors

	if err := validate.Struct(g); err != nil {
		validationErrors = append(validationErrors, (err.(validator.ValidationErrors))...)
	}

	if vErrors := g.crossValidate(); vErrors != nil {
		validationErrors = append(validationErrors, vErrors...)
	} else {
		return nil, nil
	}

	return fieldErrorsToStringMap(validationErrors), validationErrors
}

func (g *GoldResults) crossValidate() validator.ValidationErrors {
	if g.Issue != 0 {

	}
	return nil
}

// Result is used by DMResults hand holds the individual result of one test.
type Result struct {
	Key     map[string]string `json:"key"      validate:"required"`
	Options map[string]string `json:"options"  validate:"required"`
	Digest  string            `json:"md5"      validate:"required"`
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
