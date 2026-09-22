package logs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractLogSnippet(t *testing.T) {
	lines := []string{
		"uninteresting log 1",
		"uninteresting log 2",
		"uninteresting log 3",
		"uninteresting log 4",
		"uninteresting log 5",
		"uninteresting log 6",
		"uninteresting log 7",
		"error: connection refused",
		"uninteresting log 9",
		"uninteresting log 10",
		"uninteresting log 11",
		"uninteresting log 12",
		"uninteresting log 13",
		"uninteresting log 14",
		"uninteresting log 15",
	}

	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 2, 3, 100, 0))
	require.Equal(t, `==== Lines 6-10 of 15 ====
 6 | uninteresting log 6
 7 | uninteresting log 7
 8 | error: connection refused
 9 | uninteresting log 9
10 | uninteresting log 10
==== Lines 13-15 of 15 ====
13 | uninteresting log 13
14 | uninteresting log 14
15 | uninteresting log 15
`, snippet)
}

func TestExtractLogSnippet_NoTrailer(t *testing.T) {
	lines := []string{
		"uninteresting log 1",
		"uninteresting log 2",
		"uninteresting log 3",
		"uninteresting log 4",
		"uninteresting log 5",
		"uninteresting log 6",
		"uninteresting log 7",
		"error: connection refused",
		"uninteresting log 9",
		"uninteresting log 10",
		"uninteresting log 11",
		"uninteresting log 12",
		"uninteresting log 13",
		"uninteresting log 14",
		"uninteresting log 15",
	}

	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 2, 0, 100, 0))
	require.Equal(t, `==== Lines 6-10 of 15 ====
 6 | uninteresting log 6
 7 | uninteresting log 7
 8 | error: connection refused
 9 | uninteresting log 9
10 | uninteresting log 10
`, snippet)
}

func TestExtractLogSnippet_NoMatches(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf("uninteresting log %d", i+1)
	}
	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 2, 3, 100, 0))
	require.Equal(t, `==== Lines 998-1000 of 1000 ====
 998 | uninteresting log 998
 999 | uninteresting log 999
1000 | uninteresting log 1000
`, snippet)
}

func TestExtractLogSnippet_OverlappingRanges(t *testing.T) {
	lines := []string{
		"uninteresting log 1",
		"uninteresting log 2",
		"error 1",
		"uninteresting log 4",
		"error 2",
		"uninteresting log 6",
		"uninteresting log 7",
		"uninteresting log 8",
	}
	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 2, 3, 100, 0))
	require.Equal(t, `==== Lines 1-8 of 8 ====
1 | uninteresting log 1
2 | uninteresting log 2
3 | error 1
4 | uninteresting log 4
5 | error 2
6 | uninteresting log 6
7 | uninteresting log 7
8 | uninteresting log 8
`, snippet)
}

func TestExtractLogSnippet_ErrorAtBeginning(t *testing.T) {
	lines := []string{
		"error at start",
		"uninteresting log 2",
		"uninteresting log 3",
		"uninteresting log 4",
		"uninteresting log 5",
	}
	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 2, 0, 100, 0))
	require.Equal(t, `==== Lines 1-3 of 5 ====
1 | error at start
2 | uninteresting log 2
3 | uninteresting log 3
`, snippet)
}

func TestExtractLogSnippet_ErrorAtEnd(t *testing.T) {
	lines := []string{
		"uninteresting log 1",
		"uninteresting log 2",
		"uninteresting log 3",
		"uninteresting log 4",
		"error at end",
	}
	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 2, 0, 100, 0))
	require.Equal(t, `==== Lines 3-5 of 5 ====
3 | uninteresting log 3
4 | uninteresting log 4
5 | error at end
`, snippet)
}

func TestExtractLogSnippet_ShortLog(t *testing.T) {
	lines := []string{
		"uninteresting log 1",
		"uninteresting log 2",
	}
	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 2, 3, 100, 0))
	require.Equal(t, `==== Lines 1-2 of 2 ====
1 | uninteresting log 1
2 | uninteresting log 2
`, snippet)
}

func TestExtractLogSnippet_Deduplication(t *testing.T) {
	lines := []string{
		"uninteresting log 1",
		"error: connection refused", // first occurrence
		"uninteresting log 3",
		"error: connection refused", // duplicate, should be skipped
		"uninteresting log 5",
	}

	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 0, 0, 100, 0))
	require.Equal(t, `==== Lines 2-2 of 5 ====
2 | error: connection refused
`, snippet)
}

func TestExtractLogSnippet_Truncation(t *testing.T) {
	lines := []string{
		"uninteresting log 1",
		"uninteresting log 2",
		"uninteresting log 3",
		"error: connection refused", // Index 3
		"uninteresting log 5",
		"uninteresting log 6",
		"uninteresting log 7",
	}

	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 2, 0, 3, 0))
	require.Equal(t, `==== Lines 4-6 of 7 ====
4 | error: connection refused
5 | uninteresting log 5
6 | uninteresting log 6
`, snippet)
}

func TestExtractLogSnippet_MaxSnippets(t *testing.T) {
	lines := []string{
		"error 1", // Index 0
		"uninteresting log 2",
		"uninteresting log 3",
		"error 2", // Index 3
		"uninteresting log 5",
		"uninteresting log 6",
		"error 3", // Index 6
		"uninteresting log 8",
	}

	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 0, 0, 100, 2))
	require.Equal(t, `==== Lines 4-4 of 8 ====
4 | error 2
==== Lines 7-7 of 8 ====
7 | error 3
`, snippet)
}

func TestIsLogLineInteresting(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		// Match cases for "error"
		{"error: connection refused", true},
		{"An error occurred", true},
		{"panic: fatal error", true},
		{"ERROR: build failed", true},
		{"This is an ERROR line", true},

		// Match cases for "fail"
		{"fail to compile", true},
		{"compilation failed", true},
		{"FAIL: test failed", true},
		{"step failure", true},

		// Match cases for "fatal"
		{"fatal: division by zero", true},
		{"FATAL error", true},
		{"a fatal exception", true},

		// Match cases for sanitizers
		{"WARNING: ThreadSanitizer: data race (pid=18462)", true},
		{"SUMMARY: ThreadSanitizer: data race (dm:arm64+0x101979954) in dawn::native::ObjectBase::GetDevice() const+0x1c", true},
		{"ERROR: AddressSanitizer: heap-use-after-free", true},

		// Non-matching cases
		{"terror in the city", false},
		{"start unit test TestErrorCases", false},
		{"start unit test Fix ThreadSanitizer bug2345", false},
	}

	for _, tc := range tests {
		t.Run(tc.line, func(t *testing.T) {
			got := IsLogLineInteresting(tc.line)
			require.Equal(t, tc.want, got, "Line: %q", tc.line)
		})
	}
}

func TestExtractLogSnippet_TSAN(t *testing.T) {
	lines := []string{
		"uninteresting log 1",
		"start unit test Compute_ClearOrderingScratchBuffers",
		"==================",
		"WARNING: ThreadSanitizer: data race (pid=18462)",
		"  Read of size 8 at 0x00010df71010 by thread T1282:",
		"    #0 dawn::native::ObjectBase::GetDevice() const",
		"  Previous write of size 8 at 0x00010df71010 by main thread:",
		"    #0 dawn::native::ApiObjectBase::ApiObjectBase()",
		"    #1 main",
		"  Location is heap block of size 936 at 0x00010df71000 allocated by main thread:",
		"    #0 operator new(unsigned long)",
		"    #1 main",
		"  Thread T1282 (tid=220181, running) is a GCD worker thread",
		"",
		"SUMMARY: ThreadSanitizer: data race in dawn::native::ObjectBase::GetDevice() const",
		"==================",
		"[1283/1570] unit test Compute_ClearOrderingScratchBuffers done",
		"uninteresting log 18",
		"uninteresting log 19",
		"uninteresting log 20",
	}

	// Even with contextLines=1, the entire block from WARNING: ThreadSanitizer
	// through SUMMARY: ThreadSanitizer (plus 1 context line on each side) should
	// be extracted in a single contiguous range.
	snippet := RenderLineRanges(lines, ExtractSnippets(lines, 1, 0, 100, 0))
	require.Equal(t, `==== Lines 3-16 of 20 ====
 3 | ==================
 4 | WARNING: ThreadSanitizer: data race (pid=18462)
 5 |   Read of size 8 at 0x00010df71010 by thread T1282:
 6 |     #0 dawn::native::ObjectBase::GetDevice() const
 7 |   Previous write of size 8 at 0x00010df71010 by main thread:
 8 |     #0 dawn::native::ApiObjectBase::ApiObjectBase()
 9 |     #1 main
10 |   Location is heap block of size 936 at 0x00010df71000 allocated by main thread:
11 |     #0 operator new(unsigned long)
12 |     #1 main
13 |   Thread T1282 (tid=220181, running) is a GCD worker thread
14 |
15 | SUMMARY: ThreadSanitizer: data race in dawn::native::ObjectBase::GetDevice() const
16 | ==================
`, snippet)
}
