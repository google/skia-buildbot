package common

import (
	"sort"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"
)

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

var extraBugLabels = map[string][]string{
	"skcodec_scale": []string{"Area-ImageDecoder"},
	"skcodec_mode":  []string{"Area-ImageDecoder"},
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

func ExtraBugLabels(category string) []string {
	return extraBugLabels[category]
}

func ReplicationArgs(category string) string {
	return strings.Join(argsAfterExecutable[category], " ")
}

func HasCategory(c string) bool {
	_, found := prettyFuzzCategories[c]
	return found
}

// SafeParseInt parses a string that is known to contain digits into an int.
// It may fail if the number is larger than MAX_INT, but it is unlikely those
// numbers would come up in the stacktraces.
func SafeAtoi(n string) int {
	if i, err := strconv.Atoi(n); err != nil {
		glog.Errorf("Could not parse number from known digits %q: %v", n, err)
		return 0
	} else {
		return i
	}
}
