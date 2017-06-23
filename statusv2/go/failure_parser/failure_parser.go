package failure_parser

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/window"
)

var (
	// Regular expressions used for extracting failures from task logs.
	// TODO(borenet): Order most specific to least, and ensure that no
	// extracted failures overlap.

	compile1 = regexp.MustCompile(`(?m)^FAILED:.*\n(?:.*\n)+?((?:.*: error:(?:.*\n)+?)+?)(?:\[\d+/\d+\].*|\d+ errors generated.|ninja: build stopped: subcommand failed.)`)
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
					sklog.Warningf("Got != 1 submatch: %v", m[1:])
					return []string{}
				}
				matches2 := compile2.FindAllStringSubmatchIndex(m[1], -1)
				startIdx := 0
				endIdx := 0
				for _, m2 := range matches2 {
					if len(m2) != 2 {
						sklog.Warningf("Got != 1 submatch: %v", m2[1:])
						return []string{}
					}
					startIdx = endIdx
					endIdx = m2[0]
					if endIdx != startIdx {
						rv = append(rv, m[1][startIdx:endIdx])
					}
				}
				if endIdx != startIdx {
					rv = append(rv, m[1][startIdx:endIdx])
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
		regexFailure(`(?m)^(=== RUN\s+\w+\n--- FAIL: (?:.*\n)*FAIL\s+.*$)`),
		regexFailure(`(?m)^(=== RUN\s+\w+\npanic.*(?:.*\n)*FAIL\s+.*$)`),

		// Exception.
		regexFailure(`(?m)(Caught exception.*\n(.*\n)*step returned non-zero exit code.*)$`),

		// Assert.
		regexFailure(`(?m)(^.*fatal error:.*assert.*$)`),

		// Crash?
		regexFailure(`(?m)\d+(?:\.\d+)?[ms] elapsed.*\n(?:\t.*\n)?(step returned non-zero exit code: .*)`),
		regexFailure(`(?m)\+ echo \d+\n(step returned non-zero exit code: .*)`),

		// Python stacktrace.
		regexFailure(`(?m)(Traceback \(most recent call last\):.*\n(?:  File .*\n    .*\n)+?.*Error:.*)`),

		// GN error.
		regexFailure(`(?m)(ERROR at(?:.*\n)+step returned non-zero exit code:.*)`),

		// Golang log error.
		regexFailure(`(?m)^(\d+\s\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}.\d{3}\sE:.*)`),

		// Merge failure.
		regexFailure(`(?m)(.*Your local changes to the following files would be overwritten by merge(?:.*\n)*step returned non-zero exit.*)`),

		// Timed out waiting for device.
		regexFailure(`(?m)(- waiting for device -)\z`),

		// Device offline.
		regexFailure(`(?m)(error: device offline)$`),

		// Skia CT failures.
		regexFailure(`(?m)\+-{91}\+\n(?:.*\n)*?(.*failed.*)(?:.*\n)*\+-{91}\+`),
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

type failureFinder func(string) []string

func regexFailure(expr string) failureFinder {
	re := regexp.MustCompile(expr)
	return func(s string) []string {
		failures := []string{}
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			if len(m) != 2 {
				sklog.Warningf("regexp found more than one submatch!")
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
	Task *db.Task
}

// FailureParser is a struct used for parsing failure messages from Swarming
// task logs.
type FailureParser struct {
	cache    db.TaskCache
	db       db.RemoteDB
	ranOnce  bool
	swarming swarming.ApiClient
}

// New returns a FailureParser instance.
func New(taskDb db.RemoteDB) (*FailureParser, error) {
	w, err := window.New(24*time.Hour, 0, nil)
	if err != nil {
		return nil, err
	}
	cache, err := db.NewTaskCache(taskDb, w)
	if err != nil {
		return nil, err
	}
	httpClient, err := auth.NewClient(false, "google_storage_token.data", swarming.AUTH_SCOPE)
	if err != nil {
		return nil, err
	}
	s, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		return nil, err
	}
	return &FailureParser{
		cache:    cache,
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
				StrippedMessage: stripFailureMsg(f),
				OrigMessage:     f,
			})
		}
	}
	return failures
}

// Download task logs, parse them for failures.
func (fp *FailureParser) GetFailuresFromTask(t *db.Task) ([]*Failure, error) {
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

// Load newly-finished tasks and extract Failures from the ones which failed.
func (fp *FailureParser) Tick() error {
	var tasks []*db.Task
	if fp.ranOnce {
		modTasks, err := fp.cache.UpdateAndReturnModified()
		if err != nil {
			return err
		}
		tasks = make([]*db.Task, 0, len(modTasks))
		for _, t := range modTasks {
			if t.Done() && !t.Success() {
				tasks = append(tasks, t)
			}
		}
	} else {
		allTasks, err := fp.cache.GetTasksFromDateRange(time.Time{}, time.Now())
		if err != nil {
			return err
		}
		tasks = []*db.Task{}
		for _, t := range allTasks {
			if t.Done() && !t.Success() {
				tasks = append(tasks, t)
			}
		}
		fp.ranOnce = true
	}

	// Remove tasks we don't care about.
	filteredTasks := make([]*db.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Name == "Google3-Autoroller" {
			continue
		}
		// TODO(borenet): Re-enable these. Filtered because they're hard to parse.
		if t.Status == db.TASK_STATUS_MISHAP {
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
	lv := metrics2.NewLiveness("last-successful-failure-parsing")
	go util.RepeatCtx(1*time.Minute, ctx, func() {
		if err := fp.Tick(); err != nil {
			sklog.Errorf("Failed to parse failures: %s", err)
		} else {
			lv.Reset()
		}
	})
}
