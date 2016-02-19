package common

import "sort"

const (
	TEST_HARNESS_NAME = "fuzz"

	UNKNOWN_FUNCTION = "UNKNOWN"
	UNKNOWN_FILE     = "UNKNOWN"
	UNKNOWN_LINE     = -1

	ASAN_OPTIONS = "ASAN_OPTIONS=detect_leaks=0 symbolize=1 allocator_may_return_null=1"
)

var prettyFuzzCategories = map[string]string{
	"api_parse_path": "API - ParsePath",
	"skcodec_scale":  "SkCodec (Scaling)",
	"skcodec_mode":   "SkCodec (Modes)",
	"skpicture":      "SkPicture",
}

var FUZZ_CATEGORIES = []string{}

func init() {
	for k, _ := range prettyFuzzCategories {
		FUZZ_CATEGORIES = append(FUZZ_CATEGORIES, k)
	}
	sort.Strings(FUZZ_CATEGORIES)
}

func PrettifyCategory(category string) string {
	return prettyFuzzCategories[category]
}

func HasCategory(c string) bool {
	_, found := prettyFuzzCategories[c]
	return found
}
