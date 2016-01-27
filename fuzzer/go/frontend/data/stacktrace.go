package data

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/common"
)

type StackTrace struct {
	Frames []StackTraceFrame `json:"frames"`
}

type StackTraceFrame struct {
	PackageName  string `json:"packageName"`
	FileName     string `json:"fileName"`
	LineNumber   int    `json:"lineNumber"`
	FunctionName string `json:"functionName"`
}

// The `?:` at the beginning of the groups prevent them from being captured
// \1 is the "package", \2 is the file name, \3 is the line number, \4 is the function symbol string
var stackTraceLine = regexp.MustCompile(`(?:\.\./)+(?P<package>(?:\w+/)+)(?P<file>.+):(?P<line>\d+).*\(_(?P<symbol>.*)\)`)

// Given the files of a dump file (created through get_stack_trace [which uses catchsegv]), return the stack trace
func ParseStackTrace(contents string) StackTrace {
	r := bytes.NewBufferString(contents)
	scan := bufio.NewScanner(r)

	hasBegun := false

	frames := make([]StackTraceFrame, 0, 5)

	for scan.Scan() {
		line := scan.Text()
		if strings.Contains(line, "Backtrace") {
			hasBegun = true
		}
		if strings.Contains(line, "Memory map") {
			break
		}
		if !hasBegun {
			continue
		}

		if match := stackTraceLine.FindStringSubmatch(line); match != nil {
			// match[0] is the entire matched portion, [1] is the "package", [2] is the file name,
			// [3] is the line number and [4] is the encoded function symbol string.
			newFrame := FullStackFrame(match[1], match[2], decodeFunctionName(match[4]), safeParseInt(match[3]))
			frames = append(frames, newFrame)
		}
	}
	return StackTrace{Frames: frames}
}

// safeParseInt parses a string that is known to contain digits into an int.
// It may fail if the number is larger than MAX_INT, but it is unlikely those
// numbers would come up in the stacktraces.
func safeParseInt(n string) int {
	if i, err := strconv.Atoi(n); err != nil {
		glog.Errorf("Could not parse number from known digits %q: %v", n, err)
		return 0
	} else {
		return i
	}
}

var staticStart = regexp.MustCompile(`^ZL(\d+)`)
var nonstaticStart = regexp.MustCompile(`^Z(\d+)`)

var methodStart = regexp.MustCompile(`^(ZNK?)(\d+)`)
var methodName = regexp.MustCompile(`^(\d+)`)

// decodeFunctionName parses the symbol string from a stacktrace to
// get the name of the related function.
//TODO(kjlubick) parse arguments if that helps the readability of the stacktraces
// Here are some examples of symbol strings and what the various chars mean:
// (Parameters are after the names, but are unparsed as of yet)
// ZL12convert_to_8 -> ZL12 -> static function 12 long "convert_to_8"
// Z9tool_mainiPPc -> non-static function, 9 long "tool_main"
// ZN14SkBmpMaskCodec10decodeRows -> ZN14 -> type is 14 long, method name is 10 long "decodeRows"
// ZNK2DM6SKPSrc4drawEP8SkCanvas -> ZNK2 -> type is 2 long (method is konstant) "DM" ->
//    6 -> Sub type is 6 long "SKPSrc" -> 4 -> method is 4 long "draw"
//    (since there is no number directly after the 3rd step, we have found the param boundary).
func decodeFunctionName(s string) string {
	if match := methodStart.FindStringSubmatch(s); match != nil {
		length := safeParseInt(match[2])
		// ZNK? is 2-3 chars, so slice (num letters + num digits + number of spaces) chars off
		// the beginning.
		s = s[len(match[1])+len(match[2])+length:]
		f := ""
		// We look at the beginning of our trimmed string for numbers.
		// if there are numbers, we pull off a piece of the name and scan again.
		// If there is more than one piece, we separate it with ::, because it is a nested type
		// or enum thing.
		for match := methodName.FindStringSubmatch(s); match != nil; match = methodName.FindStringSubmatch(s) {
			if f != "" {
				f += "::"
			}
			length = safeParseInt(match[1])
			start := len(match[1])
			f += s[start : start+length]
			s = s[start+length:]
		}
		return f
	}
	if match := staticStart.FindStringSubmatch(s); match != nil {
		length := safeParseInt(match[1])
		// ZL is 2 chars, so advance 2 spaces + how many digits there are
		start := 2 + len(match[1])
		return s[start : start+length]
	}
	if match := nonstaticStart.FindStringSubmatch(s); match != nil {
		length := safeParseInt(match[1])
		// Z is 1 char, so advance 1 space + how many digits there are
		start := 1 + len(match[1])
		return s[start : start+length]
	}
	return common.UNKNOWN_FUNCTION
}

// FullStackFrame creates a StackTraceFrame with all components
func FullStackFrame(packageName, fileName, functionName string, lineNumber int) StackTraceFrame {
	return StackTraceFrame{
		PackageName:  packageName,
		FileName:     fileName,
		LineNumber:   lineNumber,
		FunctionName: functionName,
	}
}

func (f *StackTraceFrame) String() string {
	return fmt.Sprintf("%s%s:%d %s", f.PackageName, f.FileName, f.LineNumber, f.FunctionName)
}

func (st *StackTrace) String() string {
	s := fmt.Sprintf("StackTrace with %d frames:\n", len(st.Frames))
	for _, f := range st.Frames {
		s += fmt.Sprintf("\t%s\n", f.String())
	}
	return s
}

func (st *StackTrace) IsEmpty() bool {
	return len(st.Frames) == 0
}
