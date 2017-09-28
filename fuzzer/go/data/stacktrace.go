package data

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

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
var segvStackTraceLine = regexp.MustCompile(`(?:\.\./)+(?P<package>(?:\w+/)+)(?P<file>.+):(?P<line>\d+).*\(_(?P<symbol>.*)\)`)

// Occasionally, unsymbolized outputs sneak through.  We give it a best effort to parse.
var segvStackTraceLineUnsymbolized = regexp.MustCompile(`\(_(?P<symbol>Z.*)\)`)

// parseCatchsegvStackTrace takes the contents of a dump file of a catchsegv run, and returns the
// parsed stacktrace
func parseCatchsegvStackTrace(contents string) StackTrace {
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
		if match := segvStackTraceLine.FindStringSubmatch(line); match != nil {
			// match[0] is the entire matched portion, [1] is the "package", [2] is the file name,
			// [3] is the line number and [4] is the encoded function symbol string.
			newFrame := FullStackFrame(match[1], match[2], catchsegvFunctionName(match[4]), common.SafeAtoi(match[3]))
			frames = append(frames, newFrame)
		} else if match := segvStackTraceLineUnsymbolized.FindStringSubmatch(line); match != nil {
			newFrame := FullStackFrame(common.UNSYMBOLIZED_RESULT, common.UNSYMBOLIZED_RESULT, catchsegvFunctionName(match[1]), -1)
			frames = append(frames, newFrame)
		}
	}
	return StackTrace{Frames: frames}
}

var staticStart = regexp.MustCompile(`^ZL(\d+)`)
var nonstaticStart = regexp.MustCompile(`^Z(\d+)`)

var methodStart = regexp.MustCompile(`^(ZNK?)(\d+)`)
var methodName = regexp.MustCompile(`^(\d+)`)

// catchsegvFunctionName parses the symbol string from a stacktrace to
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
func catchsegvFunctionName(s string) string {
	if match := methodStart.FindStringSubmatch(s); match != nil {
		length := common.SafeAtoi(match[2])
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
			length = common.SafeAtoi(match[1])
			start := len(match[1])
			f += s[start : start+length]
			s = s[start+length:]
		}
		return f
	}
	if match := staticStart.FindStringSubmatch(s); match != nil {
		length := common.SafeAtoi(match[1])
		// ZL is 2 chars, so advance 2 spaces + how many digits there are
		start := 2 + len(match[1])
		return s[start : start+length]
	}
	if match := nonstaticStart.FindStringSubmatch(s); match != nil {
		length := common.SafeAtoi(match[1])
		// Z is 1 char, so advance 1 space + how many digits there are
		start := 1 + len(match[1])
		return s[start : start+length]
	}
	return common.UNKNOWN_FUNCTION
}

// The `?:` at the beginning of the groups prevent them from being captured
// \1 is the (hopefully symbolized) function name, \2 is the "package", \3 is the file name,
// \4 is the line number
var asanStackTraceLine = regexp.MustCompile(`in (?P<function>[a-zA-Z0-9_:]+).*(?:\.\./)+(?P<package>(?:\w+/)+)(?P<file>.+?):(?P<line>\d+)`)

var asanAssemblyStackTraceLine = regexp.MustCompile(`in (?P<function>[a-zA-Z0-9_:]+) \(`)

// parseCatchsegvStackTrace takes the output of an AddressSanitizer crash, and returns the parsed
// StackTrace, if it is able to find one.  If the result is not symbolized, this will return
// an empty StackTrace.
func parseASANStackTrace(contents string) StackTrace {
	r := bytes.NewBufferString(contents)
	scan := bufio.NewScanner(r)
	frames := make([]StackTraceFrame, 0, 5)
	hasBegun := false

	for scan.Scan() {
		line := scan.Text()
		if strings.Contains(line, "ERROR: AddressSanitizer:") {
			hasBegun = true
			continue
		}
		if hasBegun && line == "" {
			break
		}
		if !hasBegun {
			continue
		}

		line = strings.Replace(line, "(anonymous namespace)::", "", -1)

		if match := asanStackTraceLine.FindStringSubmatch(line); match != nil {
			// match[0] is the entire matched portion, [1] is the function name [2] is the
			// "package", [3] is the file name, [4] is the line number
			newFrame := FullStackFrame(match[2], match[3], match[1], common.SafeAtoi(match[4]))
			frames = append(frames, newFrame)
		} else if match := asanAssemblyStackTraceLine.FindStringSubmatch(line); match != nil {
			// match[1] is the function name.
			newFrame := FullStackFrame("", common.ASSEMBLY_CODE_FILE, match[1], common.UNKNOWN_LINE)
			frames = append(frames, newFrame)
		}
	}
	return StackTrace{Frames: frames}
}

var asanSummaryLine = regexp.MustCompile(`SUMMARY.*(?:\.\./)+(?P<package>(?:\w+/)+)(?P<file>.+?):(?P<line>\d+) ?(?P<function>[a-zA-Z0-9_:]+)?`)

func parseASANSummary(contents string) StackTrace {
	r := bytes.NewBufferString(contents)
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		line = strings.Replace(line, "(anonymous namespace)::", "", -1)

		if match := asanSummaryLine.FindStringSubmatch(line); match != nil {
			// match[0] is the entire matched portion, [1] is the
			// "package", [2] is the file name, [3] is the line number [4] is the function name
			f := common.UNKNOWN_FUNCTION
			if len(match) == 5 {
				f = match[4]
			}
			if f == "" {
				f = common.UNKNOWN_FUNCTION
			}
			newFrame := FullStackFrame(match[1], match[2], f, common.SafeAtoi(match[3]))
			return StackTrace{Frames: []StackTraceFrame{newFrame}}
		}
	}
	return StackTrace{}
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
