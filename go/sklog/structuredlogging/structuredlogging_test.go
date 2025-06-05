package structuredlogging

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/util"
)

func TestSplitMessage(t *testing.T) {
	test := func(name, inp string, expectLengths []int) {
		t.Run(name, func(t *testing.T) {
			msgs := make([]string, 0, len(expectLengths))
			for msg := range splitMessage(inp) {
				msgs = append(msgs, msg)
			}
			require.Len(t, msgs, len(expectLengths), "expected %d results but got %d", len(expectLengths), len(msgs))

			for index, msg := range msgs {
				expectLen := expectLengths[index]
				require.Equal(t, expectLen, len(msg), "got incorrect length at index %d", index)
			}
		})
	}

	mkLine := func(length int) string {
		s := util.RandomString(length)
		// Ensure we don't have newlines.
		s = strings.ReplaceAll(s, "\n", "a")
		return s
	}

	test("empty", "", []int{0})

	noSplit := "blah blah\nblah\nblah"
	test("no split", noSplit, []int{len(noSplit)})

	longLine := mkLine(maxLogMessageBytes * 2)
	test("long line", longLine, []int{maxLogMessageBytes, maxLogMessageBytes})

	line1 := mkLine(maxLogMessageBytes - 10)
	line2 := mkLine(9)
	test("long but still one entry", line1+"\n"+line2, []int{len(line1) + len(line2) + 1})

	line2 = mkLine(10)
	inp := line1 + "\n" + line2
	require.Greater(t, len(inp), maxLogMessageBytes)
	test("two lines", line1+"\n"+line2, []int{len(line1), len(line2)})

	lines := []string{
		mkLine(10), mkLine(20), mkLine(30),
		mkLine(maxLogMessageBytes),
		mkLine(10), mkLine(maxLogMessageBytes - 11),
	}
	test("multiple lines", strings.Join(lines, "\n"), []int{
		len(lines[0]) + len(lines[1]) + len(lines[2]) + 2,
		len(lines[3]),
		len(lines[4]) + len(lines[5]) + 1,
	})
}
