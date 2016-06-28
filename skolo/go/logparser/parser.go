package logparser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
)

// A Parser is a function that takes the text contents of a log file and returns a ParsedLog.
// There is one Parser for any type of logs that we support.  It typically uses regular
// expressions to perform any parsing.  In the process of parsing the logs, it basically
// makes a copy of every line in the log, so be aware of that memory usage.
type Parser func(string) ParsedLog

// A ParsedLog is a view of lines in a log file.  It implements methods to allow for
// random access as well as linear iterators.
type ParsedLog interface {
	// Start sets the current line to view.
	Start(int)
	// CurrLine gets the current line.
	CurrLine() int
	// Len returns how many parsed lines there are.
	Len() int
	// ReadAndNext returns a parsed line or nil and advances to the next line.
	ReadAndNext() *sklog.LogPayload
	// ReadLine sets the line to the specified number and returns the parsed line at that location.
	ReadLine(int) *sklog.LogPayload
}

type logParser struct {
	Content   []string
	Line      int
	SplitLine *regexp.Regexp
	ParseLine func(s string) *sklog.LogPayload
}

func (p *logParser) init(r io.Reader) {
	s := bufio.NewScanner(r)
	c := ""
	for s.Scan() {
		line := s.Text()
		if p.SplitLine.MatchString(line) {
			if c != "" {
				p.Content = append(p.Content, c)
			}
			c = ""
		}
		c += line + "\n"
	}
	p.Content = append(p.Content, c)
}

func (p *logParser) Start(i int) {
	p.Line = i
}

func (p *logParser) CurrLine() int {
	return p.Line
}

func (p *logParser) Len() int {
	return len(p.Content)
}

func (p *logParser) ReadAndNext() *sklog.LogPayload {
	if r := p.ReadLine(p.Line); r != nil {
		p.Line++
		return r
	}
	return nil
}

func (p *logParser) ReadLine(i int) *sklog.LogPayload {
	if i >= 0 && i < len(p.Content) {
		p.Line = i
		s := strings.TrimSpace(p.Content[i])
		if s == "" {
			return nil
		}
		return p.ParseLine(s)
	}
	return nil
}

// syslog doesn't have a year in its logs.  We will interpret it as in "this year".
var currentYear int

func ParseSyslog(contents string) ParsedLog {
	p := &logParser{
		SplitLine: syslogLine,
		ParseLine: parseSysLog,
		Line:      0,
	}
	// Update current year any time a new parsed log is created.
	// This will hopefully minimize problems around Dec 31.
	currentYear = time.Now().Year()
	p.init(bytes.NewBufferString(contents))
	return p
}

// syslog has a format like [time(implicitly in Eastern and current year)] [hostname] [logmessage]
var syslogLine = regexp.MustCompile(`\S\S\S \d\d \d\d:\d\d:\d\d [a-z0-9\-]+ `)
var syslogParse = regexp.MustCompile(`(?s)(?P<time>\S\S\S \d\d \d\d:\d\d:\d\d) (?P<hostname>[a-z0-9\-]+) (?P<payload>.*)`)

const syslogRef = "Jan 2 15:04:05 2006"

func parseSysLog(line string) *sklog.LogPayload {
	p := sklog.LogPayload{}
	matches := syslogParse.FindStringSubmatch(line)
	if len(matches) < 4 {
		sklog.CloudLogError("syslogparser", fmt.Errorf("Problem parsing syslog line: \n%s\n", line))
		return &p
	}
	// matches[1] is time, [3] is payload
	t := fmt.Sprintf("%s %d", matches[1], currentYear)
	if parsed, err := time.ParseInLocation(syslogRef, t, time.Local); err == nil {
		p.Time = parsed
	} else {
		sklog.CloudLogError("syslogparser", fmt.Errorf("Problem parsing date: %s\n", err))
	}
	p.Payload = strings.TrimSpace(matches[3])
	p.Severity = sklog.INFO

	return &p
}

func ParsePythonLog(contents string) ParsedLog {
	p := &logParser{
		SplitLine: pythonLogLine,
		ParseLine: parsePythonLog,
		Line:      0,
	}
	p.init(bytes.NewBufferString(contents))
	return p
}

// pythonLog has a format like [thread] [time(implicitly in Eastern)] [severity] [payload]
var pythonLogLine = regexp.MustCompile(`\d+ \d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d\.\d+ \w: `)
var pythonLogParse = regexp.MustCompile(`(?s)\d+ (?P<time>\d\d\d\d-\d\d-\d\d \d\d:\d\d:\d\d\.\d+) (?P<severity>\w): (?P<payload>.*)`)

const pythonLogRef = "2006-01-02 15:04:05.000"

var pythonLogSeverity = map[string]string{
	"D": sklog.DEBUG,
	"I": sklog.INFO,
	"W": sklog.WARNING,
	"E": sklog.ERROR,
	"C": sklog.CRITICAL,
}

func parsePythonLog(line string) *sklog.LogPayload {
	p := sklog.LogPayload{}
	matches := pythonLogParse.FindStringSubmatch(line)
	if len(matches) < 4 {
		sklog.CloudLogError("pyparser", fmt.Errorf("Problem parsing pythonLog line: \n%s\n", line))
		return &p
	}
	// matches[1] is time, [2] is severity, [3] is payload
	if parsed, err := time.ParseInLocation(pythonLogRef, matches[1], time.Local); err == nil {
		p.Time = parsed
	} else {
		sklog.CloudLogError("pyparser", fmt.Errorf("Problem parsing date: %s\n", err))
	}
	p.Payload = strings.TrimSpace(matches[3])
	sev, found := pythonLogSeverity[matches[2]]
	if !found {
		sev = sklog.INFO
	}
	p.Severity = sev

	return &p
}
