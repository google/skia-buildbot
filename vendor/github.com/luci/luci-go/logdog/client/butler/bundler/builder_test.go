// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"
	. "github.com/smartystreets/goconvey/convey"
)

func parse(desc string) (*logpb.ButlerLogBundle_Entry, []*logpb.LogEntry) {
	comp := strings.Split(desc, ":")
	name, entries := comp[0], comp[1:]

	be := &logpb.ButlerLogBundle_Entry{
		Desc: &logpb.LogStreamDescriptor{
			Name: name,
		},
	}

	logs := make([]*logpb.LogEntry, len(entries))
	for idx, l := range entries {
		comp := strings.SplitN(l, "@", 2)
		key, size := comp[0], 0
		if len(comp) == 2 {
			size, _ = strconv.Atoi(comp[1])
		}

		le := &logpb.LogEntry{
			Content: &logpb.LogEntry_Text{Text: &logpb.Text{
				Lines: []*logpb.Text_Line{
					{Value: key},
				},
			}},
		}

		// Pad missing data, if requested.
		if size > 0 {
			missing := size - protoSize(le)
			if missing > 0 {
				le.GetText().Lines = append(le.GetText().Lines, &logpb.Text_Line{
					Value: strings.Repeat("!", missing),
				})
			}
		}
		logs[idx] = le
	}
	return be, logs
}

// gen generates a ButlerLogBundle_Entry based on a description string.
//
// The string goes: "a:1:2:3", where "a" is the name of the stream and
// "1", "2", and "3", are different LogEntry within the stream.
//
// Note that the generated values are not valid, as they will be missing
// several fields. This is for bundling tests only :)
//
// Optionally, the string can include a size, e.g., "a:1@1024:...". This will
// cause additional data to be generated to pad the LogEntry out to the desired
// size. This is currently an approximation, as it doesn't take into account
// tag/array size overhead of the additional data.
func gen(desc string) *logpb.ButlerLogBundle_Entry {
	be, logs := parse(desc)
	be.Logs = logs
	return be
}

func logEntryName(le *logpb.LogEntry) string {
	t := le.GetText()
	if t == nil || len(t.Lines) == 0 {
		return ""
	}
	return t.Lines[0].Value
}

// "expected" is a notation to express a bundle entry and its keys:
//   "a": a bundle entry keyed on "a".
//   "+a": a terminal bundle entry keyed on "a".
//   "a:1:2:3": a bundle entry keyed on "a" with three log entries, each keyed on
//            "1", "2", and "3" respectively.
func shouldHaveBundleEntries(actual interface{}, expected ...interface{}) string {
	bundle := actual.(*logpb.ButlerLogBundle)

	errors := []string{}
	fail := func(f string, args ...interface{}) {
		errors = append(errors, fmt.Sprintf(f, args...))
	}

	term := make(map[string]bool)
	exp := make(map[string][]string)

	// Parse expectation strings.
	for _, e := range expected {
		s := e.(string)
		if len(s) == 0 {
			continue
		}

		t := false
		if s[0] == '+' {
			t = true
			s = s[1:]
		}

		parts := strings.Split(s, ":")
		name := parts[0]
		term[name] = t

		if len(parts) > 1 {
			exp[name] = append(exp[name], parts[1:]...)
		}
	}

	entries := make(map[string]*logpb.ButlerLogBundle_Entry)
	for _, be := range bundle.Entries {
		entries[be.Desc.Name] = be
	}
	for name, t := range term {
		be := entries[name]
		if be == nil {
			fail("No bundle entry for [%s]", name)
			continue
		}
		delete(entries, name)

		if t != be.Terminal {
			fail("Bundle entry [%s] doesn't match expected terminal state (exp: %v != act: %v)",
				name, t, be.Terminal)
		}

		logs := exp[name]
		for i, l := range logs {
			if i >= len(be.Logs) {
				fail("Bundle entry [%s] missing log: %s", name, l)
				continue
			}
			le := be.Logs[i]

			if logEntryName(le) != l {
				fail("Bundle entry [%s] log %d doesn't match expected (exp: %s != act: %s)",
					name, i, l, logEntryName(le))
				continue
			}
		}
		if len(be.Logs) > len(logs) {
			for _, le := range be.Logs[len(logs):] {
				fail("Bundle entry [%s] has extra log entry: %s", name, logEntryName(le))
			}
		}
	}
	for k := range entries {
		fail("Unexpected bundle entry present: [%s]", k)
	}
	return strings.Join(errors, "\n")
}

func TestBuilder(t *testing.T) {
	Convey(`A builder`, t, func() {
		tc := testclock.New(time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC))
		b := &builder{
			template: logpb.ButlerLogBundle{
				Timestamp: google.NewTimestamp(tc.Now()),
			},
		}
		templateSize := protoSize(&b.template)

		Convey(`Is not ready by default, and has no content.`, func() {
			b.size = templateSize + 1
			So(b.ready(), ShouldBeFalse)
			So(b.hasContent(), ShouldBeFalse)

			Convey(`When exceeding the desired size with content, is ready.`, func() {
				be, _ := parse("a")
				b.size = 1
				b.setStreamTerminal(be, 0)
				So(b.ready(), ShouldBeTrue)
			})
		})

		Convey(`Has a bundleSize() and remaining value of the template.`, func() {
			b.size = 1024

			So(b.bundleSize(), ShouldEqual, templateSize)
			So(b.remaining(), ShouldEqual, 1024-templateSize)
		})

		Convey(`With a size of 1024 and a 512-byte LogEntry, has content, but is not ready.`, func() {
			b.size = 1024
			be, logs := parse("a:1@512")
			b.add(be, logs[0])
			So(b.hasContent(), ShouldBeTrue)
			So(b.ready(), ShouldBeFalse)

			Convey(`After adding another 512-byte LogEntry, is ready.`, func() {
				be, logs := parse("a:2@512")
				b.add(be, logs[0])
				So(b.ready(), ShouldBeTrue)
			})
		})

		Convey(`Has content after adding a terminal entry.`, func() {
			So(b.hasContent(), ShouldBeFalse)
			be, _ := parse("a")
			b.setStreamTerminal(be, 1024)
			So(b.hasContent(), ShouldBeTrue)
		})

		for _, test := range []struct {
			title string

			streams  []string
			terminal bool
			expected []string
		}{
			{`Empty terminal entry`,
				[]string{"a"}, true, []string{"+a"}},
			{`Single non-terminal entry`,
				[]string{"a:1"}, false, []string{"a:1"}},
			{`Multiple non-terminal entries`,
				[]string{"a:1:2:3:4"}, false, []string{"a:1:2:3:4"}},
			{`Single large entry`,
				[]string{"a:1@1024"}, false, []string{"a:1"}},
			{`Multiple terminal streams.`,
				[]string{"a:1", "b:1", "a:2", "c:1"}, true, []string{"+a:1:2", "+b:1", "+c:1"}},
			{`Multiple large non-terminal streams.`,
				[]string{"a:1@1024", "b:1@8192", "a:2@4096", "c:1"}, false, []string{"a:1:2", "b:1", "c:1"}},
		} {
			Convey(fmt.Sprintf(`Test Case: %q`, test.title), func() {
				for _, s := range test.streams {
					be, logs := parse(s)
					for _, le := range logs {
						b.add(be, le)
					}

					if test.terminal {
						b.setStreamTerminal(be, 1)
					}
				}

				Convey(`Constructed bundle matches expected.`, func() {
					islice := make([]interface{}, len(test.expected))
					for i, exp := range test.expected {
						islice[i] = exp
					}
					So(b.bundle(), shouldHaveBundleEntries, islice...)
				})

				Convey(`Calculated size matches actual.`, func() {
					So(b.bundleSize(), ShouldEqual, protoSize(b.bundle()))
				})
			})
		}
	})
}
