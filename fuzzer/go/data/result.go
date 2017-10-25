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
	Debug   BuildData
	Release BuildData
}

// BuildData represents the results of parsing a given skia build's output.
type BuildData struct {
	OutputFiles
	StackTrace StackTrace
	Flags      FuzzFlag
}

// OutputFiles are the files output by the analysis
type OutputFiles struct {
	Asan   string
	Dump   string
	StdErr string
}

// GCSPackage is a struct containing all the pieces of a fuzz that exist in Google Storage.
type GCSPackage struct {
	Name             string
	FuzzCategory     string
	FuzzArchitecture string
	Debug            OutputFiles
	Release          OutputFiles
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

	SKPICTURE_DuringRendering
)

var _GREY_FLAGS = TerminatedGracefully | TimedOut

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

	"SKPICTURE_DuringRendering",
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
	return isGrey(r.Debug.Flags, r.Release.Flags)
}

// isGrey returns true if the fuzz should be considered grey, that is, is not a real crash.
func isGrey(debugFlags, releaseFlags FuzzFlag) bool {
	// If the only flags are in the _GREY_FLAGS slice, then we should ignore this.
	// TODO(kjlubick): Possibly change this to be a full-blown, user-editable blacklist
	// as in skbug.com/5191
	// 2^n-1 for a mask like 111111111111111111
	// then XOR it with the grey flags to get a bitmask that removes the
	// grey flags from the debug/release flags
	badFlags := (2<<uint(len(_FLAG_NAMES)) - 1) ^ _GREY_FLAGS
	return debugFlags&badFlags == 0 && releaseFlags&badFlags == 0
}

// ParseGCSPackage parses the results of analysis of a fuzz and creates a FuzzResult with it.
// This includes parsing the stacktraces and computing the flags about the fuzz.
func ParseGCSPackage(g GCSPackage) FuzzResult {
	result := FuzzResult{}
	result.Debug.Asan = g.Debug.Asan
	result.Debug.Dump = g.Debug.Dump
	result.Debug.StdErr = g.Debug.StdErr
	result.Debug.StackTrace = getStackTrace(g.Debug.Asan, g.Debug.Dump)
	result.Release.Asan = g.Release.Asan
	result.Release.Dump = g.Release.Dump
	result.Release.StdErr = g.Release.StdErr
	result.Release.StackTrace = getStackTrace(g.Release.Asan, g.Release.Dump)
	result.computeFlags(g.FuzzCategory)

	return result
}

// getStackTrace creates a StackTrace output from one of the two dumps given.  It first tries to
// use the AddressSanitizer dump, with the Clang dump as a fallback.
func getStackTrace(asan, dump string) StackTrace {
	if asanCrashed(asan) {
		s := parseASANStackTrace(asan)
		if s.IsEmpty() {
			s = parseASANSummary(asan)
		}
		return s
	}
	return parseCatchsegvStackTrace(dump)
}

// computeFlags parses the raw data to set both the Debug and Release flags.
func (r *FuzzResult) computeFlags(category string) {
	r.Debug.Flags = parseAll(category, &r.Debug)
	r.Release.Flags = parseAll(category, &r.Release)
}

// parseAll looks at the three input files and parses the results, based on the category.  The
// category allows for specialized flags, like SKPICTURE_DuringRendering.
func parseAll(category string, data *BuildData) FuzzFlag {
	f := FuzzFlag(0)
	// Check for SKAbort message
	if strings.Contains(data.Asan, "fatal error") {
		f |= ASANCrashed
		f |= SKAbortHit
		if data.StackTrace.IsEmpty() {
			data.StackTrace = extractSkAbortTrace(data.Asan)
		}
	}
	if strings.Contains(data.StdErr, "fatal error") {
		f |= ClangCrashed
		f |= SKAbortHit
		if data.StackTrace.IsEmpty() {
			data.StackTrace = extractSkAbortTrace(data.StdErr)
		}
	}
	// If no sk abort message and no evidence of crashes, we either terminated gracefully or
	// timed out.
	if f == 0 && !asanCrashed(data.Asan) && !clangDumped(data.Dump) {
		if (strings.Contains(data.Asan, "[terminated]") && strings.Contains(data.StdErr, "[terminated]")) ||
			(strings.Contains(data.Asan, "Signal boring") && strings.Contains(data.StdErr, "Signal boring")) {
			return TerminatedGracefully
		}
		f := FuzzFlag(0)
		if strings.Contains(data.Asan, "AddressSanitizer failed to allocate") {
			f |= BadAlloc
			f |= ASANCrashed
		}
		if strings.Contains(data.StdErr, "std::bad_alloc") {
			f |= BadAlloc
			f |= ClangCrashed
		}
		if f != 0 {
			return f
		}
		return TimedOut
	}

	// Look for clues from the various dumps.
	f |= parseAsan(category, data.Asan)
	f |= parseCatchsegv(category, data.Dump, data.StdErr)
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
	if strings.Contains(asan, "AddressSanitizer failed to allocate") {
		f |= BadAlloc
	}
	if !asanCrashed(asan) {
		return f
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
		err := errs[0]
		if category == "skpicture" && strings.Contains(err, "Rendering") {
			f |= SKPICTURE_DuringRendering
		}
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
	if strings.Contains(err, "std::bad_alloc") {
		f |= BadAlloc
	}
	if !clangDumped(dump) && strings.Contains(err, "[terminated]") {
		return f
	}
	f |= ClangCrashed
	if strings.Contains(err, "failed assertion") {
		f |= AssertionViolated
	}
	if category == "skpicture" && strings.Contains(err, "Rendering") {
		f |= SKPICTURE_DuringRendering
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
