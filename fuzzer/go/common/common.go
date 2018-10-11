package common

import (
	"sort"
	"strings"

	"go.skia.org/infra/go/sklog"
)

const (
	TEST_HARNESS_NAME = "fuzz"

	UNKNOWN_FUNCTION    = "UNKNOWN"
	UNKNOWN_FILE        = "UNKNOWN"
	ASSEMBLY_CODE_FILE  = "[assembly code]"
	UNSYMBOLIZED_RESULT = "UNSYMBOLIZED"
	UNKNOWN_LINE        = -1

	ASAN_OPTIONS        = "ASAN_OPTIONS=detect_leaks=0 symbolize=1"
	STABLE_FUZZER       = "stable"
	EXPERIMENTAL_FUZZER = "experimental"
	FUZZER_NOT_FOUND    = "FUZZER_NOT_FOUND"

	UNCLAIMED = "<unclaimed>"
)

// The list of architectures we fuzz on
var ARCHITECTURES = []string{"linux_x64"}

// By default, allow a generous amount of RAM and set a flexible approach to timeouts.
var defaultGenerationArgs = []string{"-m", "5000", "-t", "500+"}

var commonImpl CommonImpl

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
	// ArgsAfterExecutable is a slice of arguments that come after the executable
	// and before the path to the bytes file, that will be fuzzed.
	ArgsAfterExecutable []string
	// GenerationArgs is a slice of arguments that are used to adjust timeouts/memory usage
	// when generating files.
	GenerationArgs []string
}

// fuzzers is a map of fuzzer_name -> FuzzerInfo for all registered fuzzers.  This should be a
// centralized location to add a new fuzzer, i.e. adding an entry and information here, combined
// with modifying the fuzzer-be.service should be sufficient to add a new fuzzer into the system.
var fuzzers = map[string]FuzzerInfo{
	"api_draw_functions": {
		PrettyName:          "API - CanvasDrawFunctions",
		Status:              STABLE_FUZZER,
		Groomer:             "hcm",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "DrawFunctions", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"api_gradient": {
		PrettyName:          "API - Gradients",
		Status:              STABLE_FUZZER,
		Groomer:             "fmalita",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "Gradients", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"api_image_filter": {
		PrettyName:          "API - SerializedImageFilter",
		Status:              STABLE_FUZZER,
		Groomer:             "robertphillips",
		ExtraBugLabels:      []string{"Area-ImageFilter"},
		ArgsAfterExecutable: []string{"--type", "api", "--name", "SerializedImageFilter", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"api_parse_path": {
		PrettyName:          "API - ParsePath",
		Status:              STABLE_FUZZER,
		Groomer:             "caryclark",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "ParsePath", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"api_path_measure": {
		PrettyName:          "API - PathMeasure",
		Status:              STABLE_FUZZER,
		Groomer:             "caryclark",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "PathMeasure", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"api_pathop": {
		PrettyName:          "API - PathOp",
		Status:              STABLE_FUZZER,
		Groomer:             "caryclark",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "Pathop", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"api_polyutils": {
		PrettyName:          "API - PolyUtils",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "jvanverth",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "PolyUtils", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"color_deserialize": {
		PrettyName:          "SkColorSpace - Deserialize",
		Status:              STABLE_FUZZER,
		Groomer:             "scroggo",
		ExtraBugLabels:      []string{"Area-ImageDecoder"},
		ArgsAfterExecutable: []string{"--type", "color_deserialize", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"image_filter_deserialize": {
		PrettyName:          "FilterFuzz Stub",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "reed",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "filter_fuzz", "--bytes"},
		GenerationArgs:      []string{"-m", "5000", "-t", "200+"},
	},
	"jpeg_encoder": {
		PrettyName:          "JPEG encoder",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "scroggo",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "JPEGEncoder", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"mock_gpu_canvas": {
		PrettyName:          "Canvas to mock GL backend",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "robertphillips",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "MockGPUCanvas", "--bytes"},
		// For most of the canvases, some of the initial test cases take a long time. If they
		// time out (1000+ms ), afl-fuzz aborts during startup. To keep things moving, we
		// tell afl-fuzz to set the timout to 500ms and ignore any test cases that take longer
		// than that (the plus sign after 500).
		GenerationArgs: []string{"-m", "5000", "-t", "500+"},
	},
	"n32_canvas": {
		PrettyName:          "Canvas to raster n32 backend",
		Status:              STABLE_FUZZER,
		Groomer:             "halcanary",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "RasterN32Canvas", "--bytes"},
		GenerationArgs:      []string{"-m", "5000", "-t", "500+"},
	},
	"null_canvas": {
		PrettyName:          "Canvas to null canvas backend",
		Status:              STABLE_FUZZER,
		Groomer:             "halcanary",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "NullCanvas", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"null_gl_canvas": {
		PrettyName:          "Canvas to null gl canvas backend",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "robertphillips",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "NullGLCanvas", "--bytes"},
		GenerationArgs:      []string{"-m", "5000", "-t", "500+"},
	},
	"path_deserialize": {
		PrettyName:          "SkPath deserialize",
		Status:              STABLE_FUZZER,
		Groomer:             "reed",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "path_deserialize", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"pdf_canvas": {
		PrettyName:          "Canvas to PDF backend",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "halcanary",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "PDFCanvas", "--bytes"},
		GenerationArgs:      []string{"-m", "5000", "-t", "500+"},
	},
	"png_encoder": {
		PrettyName:          "PNG encoder",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "scroggo",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "PNGEncoder", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"region_deserialize": {
		PrettyName:          "SkRegion deserialize",
		Status:              STABLE_FUZZER,
		Groomer:             "halcanary",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "region_deserialize", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"region_set_path": {
		PrettyName:          "SkRegion set_path",
		Status:              STABLE_FUZZER,
		Groomer:             "reed",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "region_set_path", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"skcodec_scale": {
		PrettyName:          "SkCodec (Scaling)",
		Status:              STABLE_FUZZER,
		Groomer:             "scroggo",
		ExtraBugLabels:      []string{"Area-ImageDecoder"},
		ArgsAfterExecutable: []string{"--type", "image_scale", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"skcodec_mode": {
		PrettyName:          "SkCodec (Modes)",
		Status:              STABLE_FUZZER,
		Groomer:             "scroggo",
		ExtraBugLabels:      []string{"Area-ImageDecoder"},
		ArgsAfterExecutable: []string{"--type", "image_mode", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"sksl2glsl": {
		PrettyName:          "SKSL Compiler (GLSL)",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "ethannicholas",
		ExtraBugLabels:      []string{},
		ArgsAfterExecutable: []string{"--type", "sksl2glsl", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"skottie_json": {
		PrettyName:          "Skottie from JSON",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "fmalita",
		ExtraBugLabels:      []string{},
		ArgsAfterExecutable: []string{"--type", "skottie_json", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"skp": {
		PrettyName:          "SKP from ReadBuffer",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "reed",
		ExtraBugLabels:      []string{},
		ArgsAfterExecutable: []string{"--type", "skp", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"textblob": {
		PrettyName:          "TextBlob deserialize",
		Status:              STABLE_FUZZER,
		Groomer:             "fmalita",
		ExtraBugLabels:      []string{},
		ArgsAfterExecutable: []string{"--type", "textblob", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
	"webp_encoder": {
		PrettyName:          "WEBP encoder",
		Status:              EXPERIMENTAL_FUZZER,
		Groomer:             "scroggo",
		ExtraBugLabels:      nil,
		ArgsAfterExecutable: []string{"--type", "api", "--name", "WEBPEncoder", "--bytes"},
		GenerationArgs:      defaultGenerationArgs,
	},
}

// FUZZ_CATEGORIES is an alphabetized list of known fuzz categories.
var FUZZ_CATEGORIES = []string{}

var ANALYSIS_TYPES = []string{"ASAN_RELEASE", "ASAN_DEBUG", "CLANG_RELEASE", "CLANG_DEBUG"}

// TODO(kjlubick): Is it necessary to have two separate lists?
var STACKTRACE_ORDER = []string{"ASAN_RELEASE", "CLANG_RELEASE", "ASAN_DEBUG", "CLANG_DEBUG"}

func init() {
	commonImpl = &defaultImpl{}
	for k := range fuzzers {
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

// CategoryReminder returns a short string to remind a human what fuzz category
// a fuzz belongs to.  For example "skcodec_scale" -> "image_scale" and
// "pdf_canvas" -> "api-PDFCanvas"
func CategoryReminder(category string) string {
	f, found := fuzzers[category]
	if !found {
		sklog.Errorf("Unknown category %s", category)
		return FUZZER_NOT_FOUND
	}
	// assume ['--type', type, {'--name', api_name}]
	cr := f.ArgsAfterExecutable[1]
	if cr == "api" {
		cr += "-" + f.ArgsAfterExecutable[3]
	}
	return cr
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

func SetMockCommon(c CommonImpl) {
	commonImpl = c
}

// Returns the Hostname of this machine. Clients should use this instead of
// os.Hostname because this function can be mocked out.
func Hostname() string {
	return commonImpl.Hostname()
}
