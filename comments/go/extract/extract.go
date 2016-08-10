// Package extract pulls comments and other useful info out of GM .cpp files.
package extract

import (
	"regexp"
	"strings"
)

// GM represents the comments for a single GM.
type GM struct {
	Comment  string
	Name     string
	Filename string
	Line     int
}

var (
	defRe = regexp.MustCompile(`^DEF_[A-Z_]+\(([a-zA-Z0-9-_]+)\s?,`)
)

const (
	// These are the various states the parser can be in after looking
	// at each line.
	OTHER = iota
	COMMENT
	IN_MULTILINE
)

func isSingleLineComment(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "//") {
		return trimmed[2:], true
	}
	return "", false
}

func isStartMultiline(s string) (string, bool) {
	if index := strings.Index(s, "/*"); index > -1 {
		return strings.TrimSpace(s[index+2:]), true
	} else {
		return "", false
	}
}

func isEndMultiline(s string) (string, bool) {
	if index := strings.Index(s, "*/"); index > -1 {
		return strings.TrimSpace(s[:index]), true
	}
	return "", false
}

func isDefLine(s string) (string, bool) {
	if match := defRe.FindAllStringSubmatch(s, 1); len(match) > 0 {
		return match[0][1], true
	}
	return "", false
}

func Extract(code, filename string) []*GM {
	ret := []*GM{}
	state := OTHER
	comment := []string{}
	for num, line := range strings.Split(code, "\n") {
		switch state {
		case OTHER:
			if s, ok := isSingleLineComment(line); ok {
				comment = append(comment, s)
				state = COMMENT
			} else if s, ok := isStartMultiline(line); ok {
				if remaining, ok := isEndMultiline(s); ok {
					s = remaining
					state = COMMENT
				} else {
					state = IN_MULTILINE
				}
				if s != "" {
					comment = append(comment, s)
				}
			}
		case IN_MULTILINE:
			if s, ok := isEndMultiline(line); ok {
				if s != "" {
					comment = append(comment, s)
				}
				state = COMMENT
			} else {
				comment = append(comment, line)
			}
		case COMMENT:
			if s, ok := isSingleLineComment(line); ok {
				comment = append(comment, s)
				state = COMMENT
			} else if s, ok := isStartMultiline(line); ok {
				comment = append(comment, s)
				state = IN_MULTILINE
			} else if name, ok := isDefLine(line); ok {
				ret = append(ret, &GM{
					Comment:  strings.Join(comment, "\n"),
					Name:     name,
					Line:     num,
					Filename: filename,
				})
				comment = []string{}
				state = OTHER
			} else {
				comment = []string{}
				state = OTHER
			}
		}
	}
	return ret
}
