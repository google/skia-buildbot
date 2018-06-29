package data

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/go/util"
)

// Represents the metadata about a crash, hopefully easing debugging.
type FuzzResult struct {
	Configs map[string]BuildData // maps config name (e.g. DEBUG_ASAN) -> BuildData
}

// BuildData represents the results of parsing a given skia build's output.
type BuildData struct {
	OutputFiles
	StackTrace StackTrace
	Flags      FuzzFlag
}

// OutputFiles are the files output by the analysis
type OutputFiles struct {
	Key     string            // An optional key, typically used for the analysis config name
	Content map[string]string // maps file name/descriptor -> content
}

// GCSPackage is a struct containing all the pieces of a fuzz that exist in Google Storage.
type GCSPackage struct {
	Name             string
	FuzzCategory     string
	FuzzArchitecture string
	// maps config (e.g. DEBUG_ASAN) to output files created by running that fuzz through the
	// config (e.g. stdout and stderr)
	Files map[string]OutputFiles
}

// A bit mask representing what happened when a fuzz ran against Skia.
type FuzzFlag int

const (
	TerminatedGracefully FuzzFlag = 1 << iota
	ClangCrashed
	ASANCrashed
	AssertionViolated
	BadAlloc
	NoStackTrace
	SKAbortHit
	TimedOut
	Other

	ASAN_GlobalBufferOverflow
	ASAN_HeapBufferOverflow
	ASAN_StackBufferOverflow
	ASAN_HeapUseAfterFree
)

// BadAlloc means Out of Memory, which is not a thing fuzzing cares about.
var _GREY_FLAGS = TerminatedGracefully | TimedOut | BadAlloc

var _FLAG_NAMES = []string{
	"TerminatedGracefully",
	"ClangCrashed",
	"ASANCrashed",
	"AssertionViolated",
	"BadAlloc",
	"NoStackTrace",
	"SKAbortHit",
	"TimedOut",
	"Other",

	"ASAN_global-buffer-overflow",
	"ASAN_heap-buffer-overflow",
	"ASAN_stack-buffer-overflow",
	"ASAN_heap-use-after-free",
}

// ToHumanReadableFlags creates a sorted slice of strings that represents the flags.  The slice
// is sorted by unicode points, as per sort.Strings().
func (f FuzzFlag) ToHumanReadableFlags() []string {
	flags := make([]string, 0)
	i := 0
	for mask := 1; mask < (2 << uint(len(_FLAG_NAMES))); mask *= 2 {
		if int(f)&mask != 0 {
			flags = append(flags, _FLAG_NAMES[i])
		}
		i++
	}
	// Front end filtering logic will expect the flags to be in alphabetical order.
	sort.Strings(flags)
	return flags
}

func (f FuzzFlag) String() string {
	return fmt.Sprintf("FuzzFlag: %016b (%d) [%s]", f, f, strings.Join(f.ToHumanReadableFlags(), " | "))
}

// IsGrey returns true if the fuzz should be considered grey, that is, is not a real crash.
func (r *FuzzResult) IsGrey() bool {
	for _, c := range r.Configs {
		if !isGrey(c.Flags) {
			return false
		}
	}
	return true
}

// isGrey returns true if the fuzz should be considered grey, that is, is not a real crash.
func isGrey(flags FuzzFlag) bool {
	// If the only flags are in the _GREY_FLAGS slice, then we should ignore this.
	// TODO(kjlubick): Possibly change this to be a full-blown, user-editable blacklist
	// as in skbug.com/5191
	// 2^n-1 for a mask like 111111111111111111
	// then XOR it with the grey flags to get a bitmask that removes the
	// grey flags from the debug/release flags
	badFlags := (2<<uint(len(_FLAG_NAMES)) - 1) ^ _GREY_FLAGS
	return flags&badFlags == 0
}

// ParseGCSPackage parses the results of analysis of a fuzz and creates a FuzzResult with it.
// This includes parsing the stacktraces and computing the flags about the fuzz.
func ParseGCSPackage(g GCSPackage) FuzzResult {
	result := FuzzResult{}
	result.Configs = map[string]BuildData{}
	for _, c := range common.ANALYSIS_TYPES {
		s := StackTrace{}
		if strings.Contains(c, "ASAN") {
			s = parseASANStackTrace(g.Files[c].Content["stderr"])
			if s.IsEmpty() {
				s = parseASANSummary(g.Files[c].Content["stderr"])
			}
		} else if strings.Contains(c, "CLANG") {
			s = parseCatchsegvStackTrace(g.Files[c].Content["stdout"])
		}
		cfg := BuildData{
			OutputFiles: g.Files[c],
			StackTrace:  s,
		}
		cfg.Flags = parseAll(g.FuzzCategory, &cfg)

		result.Configs[c] = cfg
	}

	return result
}

// parseAll looks at the three input files and parses the results, based on the category.  The
// category allows for specialized flags, like SKPICTURE_DuringRendering.
func parseAll(category string, data *BuildData) FuzzFlag {
	// SkDebugf (the main source of printing) writes to stderr
	stderr := data.Content["stderr"]
	// stdout is generally blank, except if catchsegv (used for Clang builds) catches a crash
	stdout := data.Content["stdout"]

	// Check for SKAbort message
	if strings.Contains(stderr, "fatal error") {
		if data.StackTrace.IsEmpty() {
			data.StackTrace = extractSkAbortTrace(stderr)
		}
		return SKAbortHit
	}
	if strings.Contains(data.Key, "ASAN") {
		if !asanCrashed(stderr) {
			if strings.Contains(stderr, "[terminated]") || strings.Contains(stderr, "Signal boring") {
				return TerminatedGracefully
			}
			return TimedOut
		}
	}

	if strings.Contains(data.Key, "CLANG") && !clangDumped(stdout) {
		if strings.Contains(stderr, "[terminated]") || strings.Contains(stderr, "Signal boring") {
			return TerminatedGracefully
		}
		if strings.Contains(stderr, "std::bad_alloc") {
			return BadAlloc
		}
		return TimedOut
	}

	// We know there was a crash
	f := FuzzFlag(0)
	if strings.Contains(data.Key, "ASAN") {
		f |= parseAsan(category, stderr)
	} else if strings.Contains(data.Key, "CLANG") {
		f |= parseCatchsegv(category, stdout, stderr)
	}

	if f == 0 {
		// I don't know what this means (yet).
		return Other
	}

	if data.StackTrace.IsEmpty() {
		f |= NoStackTrace
	}
	return f
}

// parseAsan returns the flags discovered while looking through the AddressSanitizer output.  This
// includes things like ASAN_GlobalBufferOverflow.
func parseAsan(category, asan string) FuzzFlag {
	f := FuzzFlag(0)
	if strings.Contains(asan, "AddressSanitizer failed to allocate") ||
		strings.Contains(asan, "exceeds maximum supported size of") {
		return BadAlloc
	}
	f |= ASANCrashed
	if strings.Contains(asan, "failed assertion") {
		f |= AssertionViolated
	}
	if strings.Contains(asan, "global-buffer-overflow") {
		f |= ASAN_GlobalBufferOverflow
	}
	if strings.Contains(asan, "heap-buffer-overflow") {
		f |= ASAN_HeapBufferOverflow
	}
	if strings.Contains(asan, "stack-buffer-overflow") {
		f |= ASAN_StackBufferOverflow
	}
	if strings.Contains(asan, "heap-use-after-free") {
		f |= ASAN_HeapUseAfterFree
	}

	// Split off the stderr that happened before the crash.
	errs := strings.Split(asan, "=================")
	if len(errs) > 0 {
		// err := errs[0]
		// An example on doing a category specific parsing
		// if category == "skpicture" && strings.Contains(err, "Rendering") {
		// 	f |= SKPICTURE_DuringRendering
		// }
	}
	return f
}

// asanCrashed returns true if the asan output is consistent with a crash.
func asanCrashed(asan string) bool {
	return strings.Contains(asan, "ERROR: AddressSanitizer:") || strings.Contains(asan, "runtime error:")
}

// parseAsan returns the flags discovered while looking through the Clang dump and standard error.
// This includes things like
func parseCatchsegv(category, dump, err string) FuzzFlag {
	f := FuzzFlag(0)
	if !clangDumped(dump) && strings.Contains(err, "[terminated]") {
		return f
	}
	f |= ClangCrashed
	if strings.Contains(err, "failed assertion") {
		f |= AssertionViolated
	}
	return f
}

// clangDumped returns true if the clang output is consistent with a crash, that is, non empty.
func clangDumped(dump string) bool {
	return strings.Contains(dump, "Register dump")
}

var skAbortStackTraceLine = regexp.MustCompile(`(?:\.\./)+(?P<package>(?:\w+/)+)(?P<file>.+):(?P<line>\d+): fatal error`)

// extractSkAbortTrace looks for the fatal error string indicative of the SKAbort termination
// and tries to pull out the stacktrace frame on which it happened.
func extractSkAbortTrace(err string) StackTrace {
	st := StackTrace{}
	if match := skAbortStackTraceLine.FindStringSubmatch(err); match != nil {
		st.Frames = append(st.Frames, FullStackFrame(match[1], match[2], common.UNKNOWN_FUNCTION, util.SafeAtoi(match[3])))
	}
	return st
}
