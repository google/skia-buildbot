// Package jsonio contains the routines necessary to consume and emit JSON to be processed
// by the Gold ingester.

package jsonio

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
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/types"
)

var (
	goldResultsFields map[string]string
	resultFields      map[string]string

	// regexp to validate if a string matches hexadecimal
	isHex = regexp.MustCompile(`^[0-9a-fA-F]+$`)
)

func init() {
	goldResultsFields = jsonNameMap(GoldResults{})
	resultFields = jsonNameMap(Result{})
}

// GoldResults is the top level structure to capture the the results of a
// rendered test to be processed by Gold.
type GoldResults struct {
	GitHash string            `json:"gitHash"`
	Key     map[string]string `json:"key"`
	Results []Result          `json:"results"`

	// These indicate the results were ingested from a TryJob.
	// ChangelistID and PatchsetID correspond to code_review.Changelist.SystemID and
	// code_review.Patchset.Order, respectively
	ChangelistID     string `json:"change_list_id"`
	PatchsetOrder    int    `json:"patch_set_order"`
	PatchsetID       string `json:"patch_set_id"`
	CodeReviewSystem string `json:"crs"`

	// TryJobID corresponds to continuous_integration.TryJob.SystemID
	TryJobID                    string `json:"try_job_id"`
	ContinuousIntegrationSystem string `json:"cis"`

	// Legacy fields for tryjob support - keep these around until a few months after Skia's dm
	// is updated to produce the new format in case we want to re-ingest old results
	// (after that, TryJob results that old probably won't matter)
	// If ChangelistID is set, these will be ignored.
	BuildBucketID      string `json:"buildbucket_build_id"`
	GerritChangelistID string `json:"issue"`
	GerritPatchset     string `json:"patchset"`
}

// Result holds the individual result of one test.
type Result struct {
	// In the event of a conflict, the key/values in Result.Key will override Options, which
	// override the GoldResults.Key.
	Key     map[string]string `json:"key"      validate:"required"`
	Options map[string]string `json:"options"  validate:"required"`
	Digest  types.Digest      `json:"md5"      validate:"required"`
}

// UpdateLegacyFields takes data from old, legacy fields and copies it into the newer equivalents.
// This lets us import older data.
func (g *GoldResults) UpdateLegacyFields() error {
	if g.ChangelistID == "" && g.GerritChangelistID == "" {
		return nil
	}
	if g.ChangelistID != "" {
		// Should be good as is
		return nil
	} else if g.GerritChangelistID != "0" && g.GerritChangelistID != "-1" {
		// Handles legacy inputs for older inputs that specified GerritChangelistID
		g.ChangelistID = g.GerritChangelistID
		g.CodeReviewSystem = "gerrit"
		g.TryJobID = g.BuildBucketID
		g.ContinuousIntegrationSystem = "buildbucket"

		if n, err := strconv.ParseInt(g.GerritPatchset, 10, 64); err != nil {
			return skerr.Wrapf(err, "invalid value for patchset: %q", g.GerritPatchset)
		} else {
			g.PatchsetOrder = int(n)
		}
	}
	return nil
}

// Validate validates the instance of GoldResult to make sure it is self consistent.
func (g *GoldResults) Validate() error {
	if g == nil {
		return skerr.Fmt("Received nil pointer for GoldResult")
	}

	jn := goldResultsFields

	if g.GitHash == "" && g.ChangelistID == "" {
		return skerr.Fmt("field %q or %q must be set.", jn["GitHash"], jn["ChangelistID"])
	}
	if g.GitHash != "" && !isHex.MatchString(g.GitHash) {
		return skerr.Fmt("field %q must be hexadecimal. Received %q", jn["GitHash"], g.GitHash)
	}
	if len(g.Key) == 0 {
		return skerr.Fmt("field %q must not be empty", jn["Key"])
	}

	if ok, err := validateParams(g.Key); !ok {
		return skerr.Wrapf(err, "field %q must not have empty keys or values", jn["Key"])
	}

	if !((noneOf(g.ContinuousIntegrationSystem, g.CodeReviewSystem, g.TryJobID, g.ChangelistID, g.PatchsetID) && g.PatchsetOrder == 0) ||
		(allOf(g.ContinuousIntegrationSystem, g.CodeReviewSystem, g.TryJobID, g.ChangelistID) && (g.PatchsetID != "" || g.PatchsetOrder > 0))) {
		return skerr.Fmt("Either all of or none of fields [%q, %q, %q, %q] and one of or none of [%q, %q] must be set",
			jn["ContinuousIntegrationSystem"], jn["CodeReviewSystem"], jn["TryJobID"], jn["ChangelistID"], jn["PatchsetOrder"], jn["PatchsetID"])
	}

	hasTopLevelName := len(g.Key[types.PrimaryKeyField]) != 0
	hasTopLevelCorpus := len(g.Key[types.CorpusField]) != 0

	for i, r := range g.Results {
		if err := r.validate(); err != nil {
			return skerr.Wrapf(err, "validating field %q index %d", jn["Results"], i)
		}
		if !hasTopLevelName && len(r.Key[types.PrimaryKeyField]) == 0 {
			return skerr.Fmt("field %q is missing key %s", jn["Key"], types.PrimaryKeyField)
		}
		if !hasTopLevelCorpus && len(r.Key[types.CorpusField]) == 0 {
			return skerr.Fmt("field %q is missing key %s", jn["Key"], types.CorpusField)
		}
	}
	return nil
}

// allOf returns true iff all of the given strings are not empty string.
func allOf(xs ...string) bool {
	for _, s := range xs {
		if s == "" {
			return false
		}
	}
	return true
}

// noneOf returns true iff all of the given strings are empty string.
func noneOf(xs ...string) bool {
	for _, s := range xs {
		if s != "" {
			return false
		}
	}
	return true
}

// validate the Result instance.
func (r *Result) validate() error {
	if r == nil {
		return skerr.Fmt("nil result")
	}
	jn := resultFields
	if len(r.Key) == 0 {
		return skerr.Fmt("field %q must not be empty", jn["Key"])
	}
	if ok, err := validateParams(r.Key); !ok {
		return skerr.Wrapf(err, "field %q must not have empty keys or values", jn["Key"])
	}
	if len(r.Options) != 0 {
		// Options are optional, so only validate them if they exist.
		if ok, err := validateParams(r.Options); !ok {
			return skerr.Wrapf(err, "field %q must not have empty keys or values", jn["Options"])
		}
	}
	if r.Digest == "" {
		return skerr.Fmt("missing digest (field %q)", jn["Digest"])
	}
	if !isHex.MatchString(string(r.Digest)) {
		return skerr.Fmt("field %q must be hexadecimal. Received %q", jn["Digest"], r.Digest)
	}
	return nil
}

// validateParams returns true if all keys and values in the map are not empty strings.
func validateParams(kvMap map[string]string) (bool, error) {
	for k, v := range kvMap {
		if strings.TrimSpace(k) == "" && strings.TrimSpace(v) == "" {
			return false, skerr.Fmt("empty key and value")
		}
		if strings.TrimSpace(k) == "" {
			return false, skerr.Fmt("empty key (with value %q)", v)
		}
		if strings.TrimSpace(v) == "" {
			return false, skerr.Fmt("empty value (with key %q)", k)
		}
	}
	return true, nil
}

// jsonNameMap returns a map that maps a field name of the given struct to
// the name specified in the json tag.
func jsonNameMap(structType interface{}) map[string]string {
	sType := reflect.TypeOf(structType)
	nFields := sType.NumField()
	ret := make(map[string]string, nFields)
	for i := 0; i < nFields; i++ {
		f := sType.Field(i)
		jsonName := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if jsonName == "" || jsonName == "-" {
			continue
		}
		ret[f.Name] = jsonName
	}
	return ret
}
