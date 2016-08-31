// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package proto

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/golang/protobuf/proto"
)

var startRE = regexp.MustCompile(`^(.*)<<\s*([_a-zA-Z]+)\s*$`)

const endREStr = `^\s*%s\s*$`

func findLeftWhitespace(s string) string {
	for i, r := range s {
		if !unicode.IsSpace(r) {
			return s[:i]
		}
	}
	return s
}

func findBytewiseLCP(a, b string) string {
	if len(a) == 0 || len(b) == 0 {
		return ""
	} else if a == b {
		return a
	}

	short := a
	if len(b) < len(a) {
		short = b
	}

	for i := 0; i < len(short); i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return short
}

// writeProtoString writes the given lines into the output writer while
// correctly escaping it. This code is heavily inspired by
// "github.com/golang/protobuf/proto/text.go".
func writeProtoStringLines(w *bytes.Buffer, skip int, lines []string) {
	// equivalent to C's isprint.
	isprint := func(c byte) bool {
		return c >= 0x20 && c < 0x7f
	}

	w.WriteByte('"')
	for lIdx, line := range lines {
		if lIdx != 0 {
			// to get a "\\n".join(lines) effect: newlines between lines, but not
			// trailing.
			w.WriteString(`\n`)
		}
		// Loop over the bytes, not the runes.
		for i := skip; i < len(line); i++ {
			// Divergence from C++: we don't escape apostrophes.
			// There's no need to escape them, and the C++ parser
			// copes with a naked apostrophe.
			switch c := line[i]; c {
			case '\n':
				w.WriteString(`\n`)
			case '\r':
				w.WriteString(`\r`)
			case '\t':
				w.WriteString(`\t`)
			case '"':
				w.WriteString(`\"`)
			case '\\':
				w.WriteString(`\\`)
			default:
				if isprint(c) {
					w.WriteByte(c)
				} else {
					fmt.Fprintf(w, "\\%03o", c)
				}
			}
		}
	}
	w.WriteByte('"')
}

// ParseMultilineStrings looks for bash-style heredocs and replaces them with
// single-line text-proto-escaped strings.
//
// This looks line by line for /<<\s*([_a-zA-Z]+)\s*$/. If this is found, the
// scanner then looks until it finds /^\s*\1\s*$/. Every line between these is
// joined like "\n".join(lines), and then printed back as an escaped proto
// string. The scanner then loops back to its initial state.
//
// Not that nothing special needs to be done for e.g.
//   some_key: "string with << angles"
//
// Such a line would be left alone, because the trailing quote (which is
// mandatory in text proto) cause the starting regex to not match.
//
// For convenience, the inner lines will be treated with the equivalent of
// python's `textwrap.dedent`; any common leading whitespace that occurs on
// every line will be removed. Although both tabs and spaces count as
// whitespace, they are not equivalent (i.e. only exactly-matching whitespace
// prefixes count)
//
// The only error this may return is if there's an open heredoc without a
// matching close marker.
//
// Example:
//   this: <<EOF
//	   would
//	   turn \ninto
//       a "single"
//     line
//   EOF
//
// Turns into the same as:
//   this: "would\nturn \\ninto\n  a \"single\"\nline"
func ParseMultilineStrings(text string) (string, error) {
	terminator := ""
	terminatorRE := (*regexp.Regexp)(nil)
	needNL := false
	findLead := true
	leadingSpace := ""
	mlineBuf := []string{}
	outBuf := bytes.Buffer{}
	outBuf.Grow(len(text))

	for _, line := range strings.SplitAfter(text, "\n") {
		if terminator == "" {
			if needNL {
				outBuf.WriteByte('\n')
				needNL = false
			}
			if mtch := startRE.FindStringSubmatch(line); mtch != nil {
				_, _ = outBuf.WriteString(mtch[1])
				terminator = mtch[2]
				terminatorRE = regexp.MustCompile(fmt.Sprintf(endREStr, regexp.QuoteMeta(terminator)))
			} else {
				outBuf.WriteString(line)
			}
		} else {
			if terminatorRE.MatchString(line) {
				writeProtoStringLines(&outBuf, len(leadingSpace), mlineBuf)
				findLead = true
				terminator = ""
				terminatorRE = nil
				needNL = true
				mlineBuf = mlineBuf[:0]
			} else {
				if findLead {
					findLead = false
					leadingSpace = findLeftWhitespace(line)
				} else {
					lead := findLeftWhitespace(line)
					if len(lead) == len(line) {
						// totally whitespace lines (or empty lines) don't count for leading
						// space calculation and will be written as just an empty line.
					} else {
						leadingSpace = findBytewiseLCP(leadingSpace, lead)
					}
				}
				mlineBuf = append(mlineBuf, strings.TrimSuffix(line, "\n"))
			}
		}
	}

	if terminator != "" {
		return "", fmt.Errorf("failed to find matching terminator %q", terminator)
	}

	return outBuf.String(), nil
}

// UnmarshalTextML behaves the same as proto.UnmarshalText, except that it
// allows for multiline strings in the manner of ParseMultilineStrings.
func UnmarshalTextML(s string, pb proto.Message) error {
	s, err := ParseMultilineStrings(s)
	if err != nil {
		return err
	}
	return proto.UnmarshalText(s, pb)
}
