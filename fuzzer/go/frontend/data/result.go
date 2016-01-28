package data

import (
	"fmt"
	"strings"
)

// Represents the metadata about a crash, hopefully easing debugging.
type FuzzResult struct {
	DebugStackTrace   StackTrace
	ReleaseStackTrace StackTrace
	DebugDump         string
	ReleaseDump       string
	DebugStdErr       string
	ReleaseStdErr     string
	Flags             FuzzFlag
}

// A bit mask representing what happened when a fuzz ran against a debug version of Skia and a release version
type FuzzFlag int

const (
	DebugCrashed FuzzFlag = 1 << iota
	ReleaseCrashed
	DebugFailedGracefully
	ReleaseFailedGracefully
	DebugAssertionViolated
	DebugBadAlloc
	ReleaseBadAlloc
	DebugTimedOut
	ReleaseTimedOut
	DebugNoStackTrace
	ReleaseNoStackTrace
	DebugOther
	ReleaseOther
)

var flagNames = []string{
	"DebugCrashed",
	"ReleaseCrashed",
	"DebugFailedGracefully",
	"ReleaseFailedGracefully",
	"DebugAssertionViolated",
	"DebugBadAlloc",
	"ReleaseBadAlloc",
	"DebugTimedOut",
	"ReleaseTimedOut",
	"DebugNoStackTrace",
	"ReleaseNoStackTrace",
	"DebugOther",
	"ReleaseOther",
}

func (f FuzzFlag) ToHumanReadableFlags() []string {
	flags := make([]string, 0)
	i := 0
	for mask := 1; mask < (2 << 16); mask *= 2 {
		if int(f)&mask != 0 {
			flags = append(flags, flagNames[i])
		}
		i++
	}
	return flags
}

func (f FuzzFlag) String() string {
	return fmt.Sprintf("FuzzFlag: %016b (%d) [%s]", f, f, strings.Join(f.ToHumanReadableFlags(), " | "))
}

func ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr string) FuzzResult {
	result := FuzzResult{
		DebugDump:         debugDump,
		DebugStackTrace:   ParseStackTrace(debugDump),
		DebugStdErr:       debugErr,
		ReleaseDump:       releaseDump,
		ReleaseStackTrace: ParseStackTrace(releaseDump),
		ReleaseStdErr:     releaseErr,
		Flags:             0, //dummy value, to be updated shortly
	}
	result.computeFlags()

	return result
}

func (r *FuzzResult) computeFlags() {
	flags := FuzzFlag(0)

	if r.DebugDump != "" {
		flags |= DebugCrashed
		if r.DebugStackTrace.IsEmpty() {
			flags |= DebugNoStackTrace
		}
	}

	if r.ReleaseDump != "" {
		flags |= ReleaseCrashed
		if r.ReleaseStackTrace.IsEmpty() {
			flags |= ReleaseNoStackTrace
		}
	}

	if r.DebugStdErr == "" && r.DebugDump == "" {
		flags |= DebugTimedOut
	} else if strings.Contains(r.DebugStdErr, "failed assertion") {
		flags |= DebugAssertionViolated
	} else if strings.Contains(r.DebugStdErr, `terminate called after throwing an instance of 'std::bad_alloc'`) {
		flags |= DebugCrashed | DebugBadAlloc
	} else if strings.Contains(r.DebugStdErr, `Success`) {
		flags |= DebugFailedGracefully
	} else if r.DebugStdErr != "" {
		flags |= DebugOther
	}

	if r.ReleaseStdErr == "" && r.ReleaseDump == "" {
		flags |= ReleaseTimedOut
	} else if strings.Contains(r.ReleaseStdErr, `terminate called after throwing an instance of 'std::bad_alloc'`) {
		flags |= ReleaseCrashed | ReleaseBadAlloc
	} else if strings.Contains(r.ReleaseStdErr, `Success`) {
		flags |= ReleaseFailedGracefully
	} else if r.ReleaseStdErr != "" {
		flags |= ReleaseOther
	}

	r.Flags = flags
}
