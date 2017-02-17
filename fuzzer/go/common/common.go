package common

import (
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/sklog"
)

const (
	TEST_HARNESS_NAME = "fuzz"

	UNKNOWN_FUNCTION    = "UNKNOWN"
	UNKNOWN_FILE        = "UNKNOWN"
	UNSYMBOLIZED_RESULT = "UNSYMBOLIZED"
	UNKNOWN_LINE        = -1

	ASAN_OPTIONS        = "ASAN_OPTIONS=detect_leaks=0 symbolize=1 allocator_may_return_null=1"
	STABLE_FUZZER       = "stable"
	EXPERIMENTAL_FUZZER = "experimental"
	FUZZER_NOT_FOUND    = "FUZZER_NOT_FOUND"

	UNCLAIMED = "<unclaimed>"
)

// The list of architectures we fuzz on
var ARCHITECTURES = []string{"linux_x64"}

// FuzzerInfo contains all the configuration needed to display, execute and analyze a fuzzer.
type FuzzerInfo struct {
	// PrettyName is the human readable name for this fuzzer.
	PrettyName string
	// Status should be STABLE_FUZZER or EXPERIMENTAL_FUZZER
	Status string
	// The Groomer is responsible for stabilizing experimental fuzzers and will be the point
	// of contact for the sheriff for triaging any bad fuzzes found in stable fuzzers.
	Groomer string
	// ExtraBugLabels are any additional labels that should be included when making a bug.
	ExtraBugLabels []string
	// ArgsAfterExecutable is a map of arguments that come after the executable
	// and before the path to the bytes file, that will be fuzzed.
	ArgsAfterExecutable []string
}

// fuzzers is a map of fuzzer_name -> FuzzerInfo for all registered fuzzers.  This should be a
// centralized location to add a new fuzzer, i.e. adding an entry and information here, combined
// with modifying the fuzzer-be.service should be sufficient to add a new fuzzer into the system.
var fuzzers = map[string]FuzzerInfo{
	"api_draw_functions": {
		PrettyName:          "API - CanvasDrawFunctions",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "hcm",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "DrawFunctions", "--bytes"},
	},
	"api_gradient": {
		PrettyName:          "API - Gradients",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "fmalita",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "Gradients", "--bytes"},
	},
	"api_image_filter": {
		PrettyName:          "API - SerializedImageFilter",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "robertphillips",
		ExtraBugLabels:      []string{"Area-ImageFilter"},
		ArgsAfterExecutable: []string{"--type", "api", "--name", "SerializedImageFilter", "--bytes"},
	},
	"api_parse_path": {
		PrettyName:          "API - ParsePath",
		Status:              STABLE_FUZZER,
		Groomer:             "caryclark",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "ParsePath", "--bytes"},
	},
	"api_pathop": {
		PrettyName:          "API - PathOp",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "caryclark",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "Pathop", "--bytes"},
	},
	"color_deserialize": {
		PrettyName:          "SkColorSpace - Deserialize",
		Status:              STABLE_FUZZER,
		Groomer:             "msarett",
		ExtraBugLabels:      []string{"Area-ImageDecoder"},
		ArgsAfterExecutable: []string{"--type", "color_deserialize", "--bytes"},
	},
	"color_icc": {
		PrettyName:          "SkColorSpace - ICC",
		Status:              STABLE_FUZZER,
		Groomer:             "msarett",
		ExtraBugLabels:      []string{"Area-ImageDecoder"},
		ArgsAfterExecutable: []string{"--type", "icc", "--bytes"},
	},
	"path_deserialize": {
		PrettyName:          "SkPath deserialize",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "reed",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "path_deserialize", "--bytes"},
	},
	"region_deserialize": {
		PrettyName:          "SkRegion deserialize",
		Status:              STABLE_FUZZER,
		Groomer:             "halcanary",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "region_deserialize", "--bytes"},
	},
	"skcodec_scale": {
		PrettyName:          "SkCodec (Scaling)",
		Status:              STABLE_FUZZER,
		Groomer:             "msarett",
		ExtraBugLabels:      []string{"Area-ImageDecoder"},
		ArgsAfterExecutable: []string{"--type", "image_scale", "--bytes"},
	},
	"skcodec_mode": {
		PrettyName:          "SkCodec (Modes)",
		Status:              STABLE_FUZZER,
		Groomer:             "msarett",
		ExtraBugLabels:      []string{"Area-ImageDecoder"},
		ArgsAfterExecutable: []string{"--type", "image_mode", "--bytes"},
	},
	"sksl2glsl": {
		PrettyName:          "SKSL Compiler (GLSL)",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "ethannicholas",
		ExtraBugLabels:      []string{},
		ArgsAfterExecutable: []string{"--type", "sksl2glsl", "--bytes"},
	},
}

// FUZZ_CATEGORIES is an alphabetized list of known fuzz categories.
var FUZZ_CATEGORIES = []string{}

func init() {
	for k, _ := range fuzzers {
		FUZZ_CATEGORIES = append(FUZZ_CATEGORIES, k)
	}
	sort.Strings(FUZZ_CATEGORIES)
}

func PrettifyCategory(category string) string {
	f, found := fuzzers[category]
	if !found {
		sklog.Errorf("Unknown category %s", category)
		return FUZZER_NOT_FOUND
	}
	return f.PrettyName
}

func ExtraBugLabels(category string) []string {
	f, found := fuzzers[category]
	if !found {
		sklog.Errorf("Unknown category %s", category)
		return nil
	}
	return f.ExtraBugLabels
}

// ReplicationArgs returns a space separated list of the args needed to replicate the crash
// of a fuzz of a given category.
func ReplicationArgs(category string) string {
	f, found := fuzzers[category]
	if !found {
		sklog.Errorf("Unknown category %s", category)
		return FUZZER_NOT_FOUND
	}
	return strings.Join(f.ArgsAfterExecutable, " ")
}

// HasCategory returns if a given string corresponds to a known fuzzer category.
func HasCategory(c string) bool {
	_, found := fuzzers[c]
	return found
}

// Status returns the status of a fuzz category (i.e. stable, experimental, etc)
func Status(c string) string {
	f, found := fuzzers[c]
	if !found {
		sklog.Errorf("Unknown category %s", c)
		return FUZZER_NOT_FOUND
	}
	return f.Status
}

// Groomer returns the groomer of a fuzz category
func Groomer(c string) string {
	f, found := fuzzers[c]
	if !found {
		sklog.Errorf("Unknown category %s", c)
		return FUZZER_NOT_FOUND
	}
	return f.Groomer
}

// Returns if fuzzer knows about a given architecture
func HasArchitecture(a string) bool {
	for _, ar := range ARCHITECTURES {
		if a == ar {
			return true
		}
	}
	return false
}

// SafeParseInt parses a string that is known to contain digits into an int.
// It may fail if the number is larger than MAX_INT, but it is unlikely those
// numbers would come up in the stack traces.
func SafeAtoi(n string) int {
	if i, err := strconv.Atoi(n); err != nil {
		sklog.Errorf("Could not parse number from known digits %q: %v", n, err)
		return 0
	} else {
		return i
	}
}
