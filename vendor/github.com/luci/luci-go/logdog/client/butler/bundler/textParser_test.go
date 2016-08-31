// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/luci/luci-go/logdog/api/logpb"
	. "github.com/smartystreets/goconvey/convey"
)

type textTestCase struct {
	title      string
	delimiter  string
	source     []string
	limit      int
	increment  time.Duration
	allowSplit bool
	closed     bool
	out        []textTestOutput
}

type textTestOutput struct {
	seq       int64
	lines     []string
	increment time.Duration
}

func (o *textTestOutput) testLines() logpb.Text {
	t := logpb.Text{}
	for _, line := range o.lines {
		delim := ""
		switch {
		case strings.HasSuffix(line, windowsNewline):
			delim = windowsNewline
		case strings.HasSuffix(line, posixNewline):
			delim = posixNewline
		}

		t.Lines = append(t.Lines, &logpb.Text_Line{
			Value:     line[:len(line)-len(delim)],
			Delimiter: delim,
		})
	}
	return t
}

func TestTextParser(t *testing.T) {
	Convey(`Using a parser test stream`, t, func() {
		s := &parserTestStream{
			now:         time.Date(2015, 1, 1, 0, 0, 0, 0, time.UTC),
			prefixIndex: 1337,
		}

		for _, tst := range []textTestCase{
			{
				title:  `Standard parsing with funky newline.`,
				source: []string{"foo\nbar\r\nbaz\n"},
				out: []textTestOutput{
					{lines: []string{"foo\n", "bar\r\n", "baz\n"}},
				},
			},

			{
				title:  `Will not parse an undelimited line when not truncating.`,
				source: []string{"foo\nbar\nbaz"},
				out: []textTestOutput{
					{lines: []string{"foo\n", "bar\n"}},
				},
			},

			{
				title:      `Will not parse dangling \r when truncating but not closed.`,
				source:     []string{"foo\r\nbar\r\nbaz\r"},
				allowSplit: true,
				out: []textTestOutput{
					{lines: []string{"foo\r\n", "bar\r\n", "baz"}},
				},
			},

			{
				title:      `Will parse dangling \r when truncating and closed.`,
				source:     []string{"foo\r\nbar\r\nbaz\r"},
				allowSplit: true,
				closed:     true,
				out: []textTestOutput{
					{lines: []string{"foo\r\n", "bar\r\n", "baz\r"}},
				},
			},

			{
				title:      `Will not increase sequence for partial lines.`,
				source:     []string{"foobar\rbaz\nq\nux"},
				limit:      3,
				allowSplit: true,
				out: []textTestOutput{
					{lines: []string{"foo"}},
					{lines: []string{"bar"}},
					{lines: []string{"\rba"}},
					{lines: []string{"z\n", "q"}},
					{seq: 1, lines: []string{"\n", "ux"}},
				},
			},

			{
				title:  `Will obey the limit if it yields a line but truncates the next.`,
				source: []string{"foo\nbar\nbaz"},
				limit:  5,
				out: []textTestOutput{
					{lines: []string{"foo\n"}},
					{seq: 1, lines: []string{"bar\n"}},
				},
			},

			{
				title:      `Can parse unicode strings.`,
				source:     []string{"TEST©\r\n©\r\n©"},
				allowSplit: true,
				closed:     true,
				out: []textTestOutput{
					{lines: []string{"TEST©\r\n", "©\r\n", "©"}},
				},
			},

			{
				title:      `Unicode string with a split two-byte character should split across boundary.`,
				source:     []string{"hA©\n"},
				allowSplit: true,
				limit:      3,
				out: []textTestOutput{
					{lines: []string{"hA"}},
					{lines: []string{"©\n"}},
				},
			},

			{
				title:      `A two-byte Unicode glyph will return nothing with a limit of 1.`,
				source:     []string{"©\n"},
				limit:      1,
				allowSplit: true,
				out:        []textTestOutput{},
			},

			{
				title:     `Multiple chunks with different timestamps across newline boundaries`,
				source:    []string{"fo", "o\nb", "ar\n", "baz\nqux\n"},
				increment: time.Second,
				out: []textTestOutput{
					{seq: 0, lines: []string{"foo\n"}},
					{seq: 1, lines: []string{"bar\n"}, increment: time.Second},
					{seq: 2, lines: []string{"baz\n", "qux\n"}, increment: 2 * time.Second},
				},
			},
			{
				title:      `Will parse end of line when closed.`,
				source:     []string{"foo\nbar\nbaz"},
				allowSplit: true,
				closed:     true,
				out: []textTestOutput{
					{lines: []string{"foo\n", "bar\n", "baz"}},
				},
			},

			{
				title:  `Will parse empty lines from sequential mixed-OS delimiters.`,
				source: []string{"\n\r\n\n\r\n\n\r\n\n\n\r\n\n\n"},
				limit:  8,
				out: []textTestOutput{
					{lines: []string{"\n", "\r\n", "\n", "\r\n", "\n"}},
					{seq: 5, lines: []string{"\r\n", "\n", "\n", "\r\n", "\n", "\n"}},
				},
			},
		} {
			if tst.limit == 0 {
				tst.limit = 1024
			}

			Convey(fmt.Sprintf(`Test case: %q`, tst.title), func() {
				p := &textParser{
					baseParser: s.base(),
				}
				c := &constraints{
					limit: tst.limit,
				}

				now := s.now
				aggregate := []byte{}
				for _, chunk := range tst.source {
					p.Append(dstr(now, chunk))
					aggregate = append(aggregate, []byte(chunk)...)
					now = now.Add(tst.increment)
				}

				c.allowSplit = tst.allowSplit
				c.closed = tst.closed

				Convey(fmt.Sprintf(`Processes source %q.`, aggregate), func() {
					for _, o := range tst.out {
						le, err := p.nextEntry(c)
						So(err, ShouldBeNil)

						So(le, shouldMatchLogEntry, s.add(o.increment).le(o.seq, o.testLines()))
					}

					le, err := p.nextEntry(c)
					So(err, ShouldBeNil)
					So(le, ShouldBeNil)
				})
			})
		}
	})
}
