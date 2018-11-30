package failure_parser

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/types"
)

var (
	// Regular expressions used for extracting failures from task logs.
	// TODO(borenet): Order most specific to least, and ensure that no
	// extracted failures overlap.

	compile1 = regexp.MustCompile(`(?m)^FAILED:(?:.*\n)+?((?:.*: error:(?:.*\n)+?)+?)(?:\[\d+/\d+\].*|\d+ errors generated.|ninja: build stopped: subcommand failed.)`)
	compile2 = regexp.MustCompile(`(?m).*: error:.*\n`)

	failureRes = []failureFinder{
		// Failed tests in DM.
		regexFailure(`(?m)^(FAILURE: .+)$`),

		// Compile failure.
		func(s string) []string {
			rv := []string{}
			matches := compile1.FindAllStringSubmatch(s, -1)
			for _, m := range matches {
				if len(m) != 2 {
					sklog.Warningf("Got != 1 submatch. Regex:\n%s\nGot:\n%v", compile1, m[1:])
					return []string{}
				}
				matches2 := compile2.FindAllStringSubmatchIndex(m[1], -1)
				for _, idx := range matches2 {
					rv = append(rv, m[1][idx[0]:idx[1]])
				}
			}
			return rv
		},

		// Compile failure, Windows.
		regexFailure(`(?m)^(FAILED:(?:.*\n)+?)(?:\[\d+/\d+\].*|\d+ errors generated.|ninja: build stopped: subcommand failed.)`),

		// Valgrind.
		regexFailure(`(?m)((?:^==\d+== .*\n)+\{(?:.*\n)+?\})`),

		// ASAN error.
		regexFailure(`(?m)(.*runtime error(?:.*\n)+?)step returned non-zero exit code.*`),

		// TSAN warning.
		regexFailure(`(?m)(==================\nWARNING:(?:.*\n)+?==================)`),

		// Segfault. Take the few lines above it for context.
		regexFailure(`(?m)((?:.*\n){0,5}Segmentation fault)`),

		// Golang tests.
		regexFailure(`(?m)^(=== RUN\s+\w+\n--- FAIL: (?:.*\n)+FAIL\s+.*$)`),
		regexFailure(`(?m)^(=== RUN\s+\w+\npanic(?:.*\n)+FAIL\s+.*$)`),

		// Exception.
		regexFailure(`(?m)(Caught exception.(?:.*\n)+step returned non-zero exit code.*)$`),

		// Assert.
		regexFailure(`(?m)(^.*fatal error:.*assert.*$)`),

		// Crash?
		regexFailure(`(?m)\d+(?:\.\d+)?[ms] elapsed.*\n(?:\t.*\n)?(step returned non-zero exit code: .*)`),
		regexFailure(`(?m)\+ echo \d+\n(step returned non-zero exit code: .*)`),

		// Python stacktrace.
		regexFailure(`(?m)(Traceback \(most recent call last\):.*\n(?:  File .*\n    .*\n)+?.*(?:Error|Exception):.*)`),

		// GN error.
		regexFailure(`(?m)(ERROR at(?:.*\n)+step returned non-zero exit code:.*)`),

		// Golang log error.
		regexFailure(`(?m)^(\d+\s\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}.\d{3}\sE:.*)`),

		// Merge failure.
		regexFailure(`(?m)(.*Your local changes to the following files would be overwritten by merge(?:.*\n)+step returned non-zero exit.*)`),

		// Timed out waiting for device.
		regexFailure(`(?m)(- waiting for device -)\z`),

		// Device offline.
		regexFailure(`(?m)(error: device offline)`),
		regexFailure(`(?m)(error: device not found)`),

		// Skia CT failures.
		regexFailure(`(?m)\+-{91}\+\n(?:.*\n)*?(.*failed.*)(?:.*\n)+\+-{91}\+`),

		// Don't understand config XYZ.
		regexFailure(`(?m)(.*Skipping config \w+: Don't understand '\w+'(?:.*\n)+?)(?:==|step returned non-zero)`),

		// glGetError.
		regexFailure(`(?m)(---- glGetError(?:.*\n)+)step returned non-zero exit code.*$`),

		// Undefined flag (golang).
		regexFailure(`(?m)(flag provided but not defined:(?:.*\n)+?FAIL.*)\n`),

		// run_unittests failures.
		regexFailure(`(?m)(={10,} TEST FAILURE ={10,}(?:.*\n)+?Full output:\n-{10,}(?:.*\n)+?-{10,})`),

		// "unrecognized import path" (golang)
		regexFailure(`(?m)((?:.*unrecognized import path.*\n)+)`),

		// Infra build failures.
		regexFailure(`(?m)(# go\.skia\.org/infra.*\n(?:.*\.go:\d+:.*\n(?:.*\n)*)+)step returned non-zero`),
	}

	// Replacements for failure messages, performed in order.
	replaceRes = []replace{
		// Convert NT paths to Unix.
		{regexp.MustCompile(`(?m)\\`), "/"},
		{regexp.MustCompile(`(?m)[a-zA-Z]:/`), "/"},

		// Trim path elements.
		{regexp.MustCompile(`(?m)/b/work/skia/`), "../"},
		{regexp.MustCompile(`(?m)/b/s/w/ir/`), "../"},
		{regexp.MustCompile(`(?m)/home/\w+/`), "../"},
		{regexp.MustCompile(`(?m)(?:\.\./)+`), ""},

		// Replace memory addresses.
		{regexp.MustCompile(`(?m)0x[a-f0-9]+`), "0xffff"},

		// Trim newline at end of text.
		{regexp.MustCompile(`(?m)\n\z`), ""},
	}
)

// failureFinder is a function which searches a log string for known failures
// and returns the substrings containing each failure.
type failureFinder func(string) []string

// regexFailure returns a failureFinder which searches the logs using a regular
// expression. The given expression should have exactly one capturing group.
func regexFailure(expr string) failureFinder {
	re := regexp.MustCompile(expr)
	if re.NumSubexp() != 1 {
		sklog.Fatalf("Regular expression has != 1 capturing group:\n%s", expr)
	}
	return func(s string) []string {
		failures := []string{}
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			if len(m) != 2 {
				sklog.Warningf("Got != 1 submatch. Regex:\n%s\nGot:\n%v", expr, m[1:])
				for _, s := range m[1:] {
					sklog.Infof(s)
				}
			} else {
				failures = append(failures, m[1])
			}
		}
		return failures
	}
}

// replace is a struct which pairs a regexp with a replacement string.
type replace struct {
	re   *regexp.Regexp
	with string
}

// Failure represents a single failure message within a task.
type Failure struct {
	// The stripped, simplified failure message. This is expected to
	// uniquely identify the failure, ie. Failures with the same
	// StrippedMessage can be assumed to be the same.
	StrippedMessage string

	// The unmodified failure message.
	OrigMessage string

	// The task which incurred the failure.
	Task *types.Task
}

// FailureParser is a struct used for parsing failure messages from Swarming
// task logs.
type FailureParser struct {
	db       db.RemoteDB
	queryId  string
	ranOnce  bool
	swarming swarming.ApiClient
}

// New returns a FailureParser instance.
func New(taskDb db.RemoteDB) (*FailureParser, error) {
	ts, err := auth.NewDefaultLegacyTokenSource(false, swarming.AUTH_SCOPE)
	if err != nil {
		return nil, err
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	s, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		return nil, err
	}
	return &FailureParser{
		db:       taskDb,
		ranOnce:  false,
		swarming: s,
	}, nil
}

// strip instance-specific things from the failure message.
func stripFailureMsg(msg string) string {
	for _, rep := range replaceRes {
		msg = rep.re.ReplaceAllString(msg, rep.with)
	}
	return msg
}

// Parse the given string for failures.
func ParseFailures(stdout string) []*Failure {
	// Find failures matching the regexes defined above.
	failures := []*Failure{}
	for _, re := range failureRes {
		for _, f := range re(stdout) {
			failures = append(failures, &Failure{
				StrippedMessage: util.Truncate(stripFailureMsg(f), 5*1024),
				OrigMessage:     util.Truncate(f, 5*1024),
			})
		}
	}
	return failures
}

// Download task logs, parse them for failures.
func (fp *FailureParser) GetFailuresFromTask(t *types.Task) ([]*Failure, error) {
	out, err := fp.swarming.GetStdoutOfTask(t.SwarmingTaskId)
	if err != nil {
		return nil, fmt.Errorf("Failed to get swarming task output: %s", err)
	}

	failures := ParseFailures(out.Output)
	for _, f := range failures {
		f.Task = t
	}
	if len(failures) == 0 {
		sklog.Warningf("Parsed no failures from https://%s/task?id=%s", swarming.SWARMING_SERVER, t.SwarmingTaskId)
	}

	return failures, nil
}

// Load newly-finished tasks.
func (fp *FailureParser) GetNewlyFinishedTasks() ([]*types.Task, error) {
	var modTasks []*types.Task
	var err error
	if fp.queryId != "" {
		modTasks, err = fp.db.GetModifiedTasks(fp.queryId)
	}
	if fp.queryId == "" || db.IsUnknownId(err) {
		if fp.queryId != "" {
			sklog.Warningf("Connection to db lost; loading all tasks from last 24 hours.")
		}
		queryId, err := fp.db.StartTrackingModifiedTasks()
		if err != nil {
			return nil, err
		}
		now := time.Now()
		start := now.Add(-24 * time.Hour)
		modTasks, err = fp.db.GetTasksFromDateRange(start, now, "")
		if err != nil {
			fp.db.StopTrackingModifiedTasks(fp.queryId)
			return nil, err
		}
		fp.queryId = queryId
	} else if err != nil {
		return nil, err
	}
	rv := make([]*types.Task, 0, len(modTasks))
	for _, t := range modTasks {
		if t.Done() && !t.Success() {
			rv = append(rv, t)
		}
	}
	return rv, nil
}

// Load newly-finished tasks and extract Failures from the ones which failed.
func (fp *FailureParser) Tick() error {
	tasks, err := fp.GetNewlyFinishedTasks()
	if err != nil {
		return err
	}

	// Remove tasks we don't care about.
	filteredTasks := make([]*types.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Name == "Google3-Autoroller" {
			continue
		}
		// TODO(borenet): Re-enable these. Filtered because they're hard to parse.
		if t.Status == types.TASK_STATUS_MISHAP {
			continue
		}
		filteredTasks = append(filteredTasks, t)
	}
	tasks = filteredTasks

	sklog.Infof("Found %d failed tasks.", len(tasks))

	failures := make(map[string][]*Failure, len(tasks))
	for _, t := range tasks {
		fails, err := fp.GetFailuresFromTask(t)
		if err != nil {
			return err
		}
		for _, f := range fails {
			failures[f.StrippedMessage] = append(failures[f.StrippedMessage], f)
		}
	}

	if len(failures) > 0 {
		sklog.Infof("Found %d failures:", len(failures))
		for msg, f := range failures {
			chars := len(msg)
			if chars > 50 {
				chars = 50
			}
			sklog.Infof("%s\nhttps://%s/task?id=%s", msg[:chars], swarming.SWARMING_SERVER, f[0].Task.SwarmingTaskId)
		}
	}
	return nil
}

// Start initiates the FailureParser's periodic poll-and-parse-failures loop.
func (fp *FailureParser) Start(ctx context.Context) {
	lv := metrics2.NewLiveness("last_successful_failure_parsing")
	go util.RepeatCtx(1*time.Minute, ctx, func() {
		if err := fp.Tick(); err != nil {
			sklog.Errorf("Failed to parse failures: %s", err)
		} else {
			lv.Reset()
		}
	})
}
