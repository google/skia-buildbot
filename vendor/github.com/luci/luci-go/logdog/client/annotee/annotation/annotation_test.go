// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package annotation

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/proto/milo"
	"github.com/luci/luci-go/logdog/common/types"

	. "github.com/smartystreets/goconvey/convey"
)

const testDataDir = "test_data"
const testExpDir = "test_expectations"

var generate = flag.Bool("annotee.generate", false, "If true, regenerate expectations from source.")

type testCase struct {
	name string
	exe  *Execution
}

func (tc *testCase) state(startTime time.Time) *State {
	cb := testCallbacks{
		closed:   map[string]struct{}{},
		logs:     map[types.StreamName][]string{},
		logsOpen: map[types.StreamName]struct{}{},
	}
	return &State{
		LogNameBase: types.StreamName("base"),
		Callbacks:   &cb,
		Clock:       testclock.New(startTime),
		Execution:   tc.exe,
	}
}

func (tc *testCase) generate(t *testing.T, startTime time.Time, touched stringset.Set) error {
	st := tc.state(startTime)
	p, err := playAnnotationScript(t, tc.name, st)
	if err != nil {
		return err
	}
	touched.Add(p)
	st.Finish()

	// Write out generated protos.
	merr := errors.MultiError(nil)

	step := st.RootStep()
	p, err = writeStepProto(tc.name, step.CanonicalName(), step.Proto())
	if err != nil {
		merr = append(merr, fmt.Errorf("Failed to write step proto for %q::%q: %v", tc.name, step.CanonicalName(), err))
	}
	touched.Add(p)

	// Write out generated logs.
	cb := st.Callbacks.(*testCallbacks)
	for logName, lines := range cb.logs {
		p, err := writeLogText(tc.name, string(logName), lines)
		if err != nil {
			merr = append(merr, fmt.Errorf("Failed to write log text for %q::%q: %v", tc.name, logName, err))
		}
		touched.Add(p)
	}

	if merr != nil {
		return merr
	}
	return nil
}

func normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return r
		}
		return '_'
	}, s)
}

func superfluous(touched stringset.Set) ([]string, error) {
	var paths []string

	files, err := ioutil.ReadDir(testExpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %v", testExpDir, err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		path := filepath.Join(testExpDir, f.Name())
		if !touched.Has(path) {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// playAnnotationScript loads named annotation script and plays it
// through the supplied State line-by-line. Returns path to the annotation
// script.
//
// Empty lines and lines beginning with "#" are ignored. Preceding whitespace
// is discarded.
func playAnnotationScript(t *testing.T, name string, st *State) (string, error) {
	tc := st.Clock.(testclock.TestClock)

	path := filepath.Join(testDataDir, fmt.Sprintf("%s.annotations.txt", normalize(name)))
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("Failed to open annotations source [%s]: %v", path, err)
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var nextErr string
	for scanner.Scan() {
		// Trim, discard empty lines and comment lines.
		line := strings.TrimLeftFunc(scanner.Text(), unicode.IsSpace)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case line == "+time":
			tc.Add(1 * time.Second)

		case strings.HasPrefix(line, "+error"):
			nextErr = strings.SplitN(line, " ", 2)[1]

		default:
			// Annotation.
			err := st.Append(line)
			if nextErr != "" {
				expectedErr := nextErr
				nextErr = ""

				if err == nil {
					return "", fmt.Errorf("expected error, but didn't encounter it: %q", expectedErr)
				}
				if !strings.Contains(err.Error(), expectedErr) {
					return "", fmt.Errorf("expected error %q, but got: %v", expectedErr, err)
				}
			} else if err != nil {
				return "", err
			}
		}
	}

	return path, nil
}

func loadStepProto(t *testing.T, test, name string) *milo.Step {
	path := filepath.Join(testExpDir, fmt.Sprintf("%s_%s.proto.txt", normalize(test), normalize(name)))
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Errorf("Failed to read milo.Step proto [%s]: %v", path, err)
		return nil
	}

	st := milo.Step{}
	if err := proto.UnmarshalText(string(data), &st); err != nil {
		t.Errorf("Failed to Unmarshal milo.Step proto [%s]: %v", path, err)
		return nil
	}
	return &st
}

func writeStepProto(test, name string, step *milo.Step) (string, error) {
	path := filepath.Join(testExpDir, fmt.Sprintf("%s_%s.proto.txt", normalize(test), normalize(name)))
	return path, ioutil.WriteFile(path, []byte(proto.MarshalTextString(step)), 0644)
}

func loadLogText(t *testing.T, test, name string) []string {
	path := filepath.Join(testExpDir, fmt.Sprintf("%s_%s.txt", normalize(test), normalize(name)))
	f, err := os.Open(path)
	if err != nil {
		t.Errorf("Failed to open log lines [%s]: %v", path, err)
		return nil
	}
	defer f.Close()

	lines := []string(nil)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func writeLogText(test, name string, text []string) (string, error) {
	path := filepath.Join(testExpDir, fmt.Sprintf("%s_%s.txt", normalize(test), normalize(name)))
	return path, ioutil.WriteFile(path, []byte(strings.Join(text, "\n")), 0644)
}

// testCallbacks implements the Callbacks interface, retaining all callback
// data in memory.
type testCallbacks struct {
	// closed is the set of steps that have been closed, keyed on step
	// CanonicalName.
	closed map[string]struct{}

	// logs is the content of emitted annotation logs, keyed on stream name.
	logs map[types.StreamName][]string
	// logsOpen tracks whether a given annotation log is open.
	logsOpen map[types.StreamName]struct{}
}

func (tc *testCallbacks) StepClosed(s *Step) {
	tc.closed[s.CanonicalName()] = struct{}{}
}

func (tc *testCallbacks) StepLogLine(s *Step, n types.StreamName, label, line string) {
	if _, ok := tc.logs[n]; ok {
		// The log exists. Assert that it is open.
		if _, ok := tc.logsOpen[n]; !ok {
			panic(fmt.Errorf("write to closed log stream: %q", n))
		}
	}

	tc.logsOpen[n] = struct{}{}
	tc.logs[n] = append(tc.logs[n], line)
}

func (tc *testCallbacks) StepLogEnd(s *Step, n types.StreamName) {
	if _, ok := tc.logsOpen[n]; !ok {
		panic(fmt.Errorf("close of closed log stream: %q", n))
	}
	delete(tc.logsOpen, n)
}

func (tc *testCallbacks) Updated(s *Step, ut UpdateType) {}

func TestState(t *testing.T) {
	t.Parallel()

	startTime := time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC)
	testCases := []testCase{
		{"default", &Execution{
			Name:    "testcommand",
			Command: []string{"testcommand", "foo", "bar"},
			Dir:     "/path/to/dir",
			Env: map[string]string{
				"FOO": "BAR",
				"BAZ": "QUX",
			},
		}},
		{"timestamps", nil},
		{"coverage", nil},
		{"nested", nil},
	}

	if *generate {
		touched := stringset.New(0)
		for _, tc := range testCases {
			if err := tc.generate(t, startTime, touched); err != nil {
				t.Fatalf("Failed to generate %q: %v\n", tc.name, err)
			}
		}

		paths, err := superfluous(touched)
		if err != nil {
			if merr, ok := err.(errors.MultiError); ok {
				for i, ierr := range merr {
					t.Logf("Error #%d: %s", i, ierr)
				}
			}
			t.Fatalf("Superflous test data: %v", err)
		}
		for _, path := range paths {
			t.Log("Removing superfluous test data:", path)
			os.Remove(path)
		}
		return
	}

	Convey(`A testing annotation State`, t, func() {
		for _, testCase := range testCases {
			st := testCase.state(startTime)

			Convey(fmt.Sprintf(`Correctly loads/generates for %q test case.`, testCase.name), func() {

				_, err := playAnnotationScript(t, testCase.name, st)
				So(err, ShouldBeNil)

				// Iterate through generated streams and validate.
				st.Finish()

				// All log streams should be closed.
				cb := st.Callbacks.(*testCallbacks)
				So(cb.logsOpen, ShouldResemble, map[types.StreamName]struct{}{})

				// Iterate over each generated stream and assert that it matches its
				// expectation. Do it deterministically so failures aren't frustrating
				// to reproduce.
				Convey(`Has correct Step value`, func() {
					rootStep := st.RootStep()

					exp := loadStepProto(t, testCase.name, rootStep.CanonicalName())
					So(rootStep.Proto(), ShouldResemble, exp)
				})

				// Iterate over each generated log and assert that it matches its
				// expectations.
				logs := make([]string, 0, len(cb.logs))
				for k := range cb.logs {
					logs = append(logs, string(k))
				}
				sort.Strings(logs)
				for _, logName := range logs {
					log := cb.logs[types.StreamName(logName)]
					exp := loadLogText(t, testCase.name, logName)
					So(log, ShouldResemble, exp)
				}
			})
		}

		Convey(`Append to a closed State is a no-op.`, func() {
			st := testCases[0].state(startTime)
			st.Finish()
			sclone := st
			So(st.Append("asdf"), ShouldBeNil)
			So(st, ShouldResemble, sclone)
		})
	})
}
