package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractFilesWithDiffs(t *testing.T) {
	test := func(name, input string, expectedFiles []string) {
		t.Run(name, func(t *testing.T) {
			actualFiles := extractFilesWithDiffs(input)
			assert.Equal(t, expectedFiles, actualFiles)
		})
	}

	test("no diffs", "\n", nil)

	const withDiffs = `:100644 100644 70423240e35dbf7c866cb44709e010e700a6ae65 0000000000000000000000000000000000000000 M	WORKSPACE
:000000 100644 0000000000000000000000000000000000000000 0000000000000000000000000000000000000000 A	bazel/external/buildifier/buildifier.go
:100644 100644 3c82f60478d2d2b7218e6a59e8561171dbf3d82e 0000000000000000000000000000000000000000 M	cmd/presubmit/BUILD.bazel
:100755 000000 d4f82380f925f434075678e7c6162ed2c8ff8099 0000000000000000000000000000000000000000 D	cmd/presubmit/test.sh`
	test("some diffs", withDiffs, []string{
		"WORKSPACE",
		"bazel/external/buildifier/buildifier.go",
		"cmd/presubmit/BUILD.bazel",
		"cmd/presubmit/test.sh",
	})

}

const (
	// The following examples were based on real output of
	// git rev-list HEAD ^origin/main --format="%D$|%P"
	// Except the commit hashes were changed to be a bit easier to read.

	// A straight line history with no merges/rebases
	oneCLWithManyCommits = `commit 7777777777777777777777777777777777777777
HEAD -> remove-recipes$|6666666666666666666666666666666666666666
commit 6666666666666666666666666666666666666666
$|5555555555555555555555555555555555555555
commit 5555555555555555555555555555555555555555
$|4444444444444444444444444444444444444444
commit 4444444444444444444444444444444444444444
$|3333333333333333333333333333333333333333
commit 3333333333333333333333333333333333333333
$|2222222222222222222222222222222222222222
commit 2222222222222222222222222222222222222222
$|1111111111111111111111111111111111111111
`
	oneCLWithOneCommit = `commit bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
HEAD -> remove-recipes$|aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`

	// This was a normal CL that the user rebased. We want to diff off of commit 0000...
	// instead of 1111.... because if we choose 1111.... we'll see all the changes that
	// landed on main that were not because of us
	oneCLWithRebase = `commit 6666666666666666666666666666666666666666
HEAD -> first$|5555555555555555555555555555555555555555 0000000000000000000000000000000000000000
commit 5555555555555555555555555555555555555555
$|4444444444444444444444444444444444444444
commit 4444444444444444444444444444444444444444
$|3333333333333333333333333333333333333333
commit 3333333333333333333333333333333333333333
$|2222222222222222222222222222222222222222
commit 2222222222222222222222222222222222222222
$|1111111111111111111111111111111111111111`

	// This is a chain of 3 CLs. At (2 points 6666... and 8888...), a child branch pulled in its
	// parent. We want to ignore these and just find the latest commit for the parent CL
	// (second_link_in_chain) which is 7777...
	chainOfThreeCLs = `commit 9999999999999999999999999999999999999999
HEAD -> third_link_in_chain$|8888888888888888888888888888888888888888
commit 8888888888888888888888888888888888888888
$|3333333333333333333333333333333333333333 7777777777777777777777777777777777777777
commit 7777777777777777777777777777777777777777
second_link_in_chain$|6666666666666666666666666666666666666666
commit 6666666666666666666666666666666666666666
$|2222222222222222222222222222222222222222 5555555555555555555555555555555555555555
commit 5555555555555555555555555555555555555555
first_link_in_chain$|4444444444444444444444444444444444444444
commit 4444444444444444444444444444444444444444
$|3333333333333333333333333333333333333333
commit 3333333333333333333333333333333333333333
$|2222222222222222222222222222222222222222
commit 2222222222222222222222222222222222222222
$|1111111111111111111111111111111111111111`

	// This has two CLs in a chain, when the first one rebased on top of origin/main
	// (commit aaaa...) and then the second one pulled in those changes from the first.
	// We want to diff off the most recent commit associated with first_link_in_chain (9999....).
	chainOfTwoCLsWithRebase = `commit 0000000000000000000000000000000000000000
HEAD -> second_link_in_chain$|8888888888888888888888888888888888888888 9999999999999999999999999999999999999999
commit 9999999999999999999999999999999999999999
first_link_in_chain$|5555555555555555555555555555555555555555 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
commit 8888888888888888888888888888888888888888
$|7777777777777777777777777777777777777777 5555555555555555555555555555555555555555
commit 5555555555555555555555555555555555555555
$|4444444444444444444444444444444444444444
commit 7777777777777777777777777777777777777777
$|6666666666666666666666666666666666666666
commit 6666666666666666666666666666666666666666
$|4444444444444444444444444444444444444444
commit 4444444444444444444444444444444444444444
$|3333333333333333333333333333333333333333
commit 3333333333333333333333333333333333333333
$|2222222222222222222222222222222222222222
commit 2222222222222222222222222222222222222222
$|1111111111111111111111111111111111111111`
)

func TestExtractBranchBase_Success(t *testing.T) {
	test := func(name, input, expectedOutput string) {
		t.Run(name, func(t *testing.T) {
			actualOutput := extractBranchBase(input)
			assert.Equal(t, expectedOutput, actualOutput)
		})
	}

	test("one CL returns last parent", oneCLWithManyCommits, "1111111111111111111111111111111111111111")
	test("CL with one commit returns parent", oneCLWithOneCommit, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	test("rebase uses most recent un-recognized commit", oneCLWithRebase, "0000000000000000000000000000000000000000")
	test("chain of CLs returns nearest branch commit", chainOfThreeCLs, "7777777777777777777777777777777777777777")
	test("chain of CLs with rebase still returns nearest branch", chainOfTwoCLsWithRebase, "9999999999999999999999999999999999999999")
	test("empty output means empty return value", "", "")
}

func TestExtractChangedAndDeletedFiles_NewDeletedModifiedFiles_AttributesLineNumbers(t *testing.T) {
	// This example has a diff of a new, deleted, and modified file.
	// This was based on actual output of
	// git diff-index origin/main --patch-with-raw --unified=0 --no-color
	const input = `:000000 100644 0000000000000000000000000000000000000000 c858dccaae4f8957fd3fab29dd930c712b490039 A	bazel/copts2.bzl
:100644 000000 d7d17e863c97dcba0b11ae5e8b2139d695f183bd 0000000000000000000000000000000000000000 D	bazel/defines.bzl
:100644 100644 f3265464e32a3b8bc1f4e4238b6c089926d35feb 0000000000000000000000000000000000000000 M	src/pathops/SkPathOpsDebug.h

diff --git a/bazel/copts2.bzl b/bazel/copts2.bzl
new file mode 100644
index 0000000000..c858dccaae
--- /dev/null
+++ b/bazel/copts2.bzl
@@ -0,0 +1,7 @@
+"""
+THIS IS THE EXTERNAL-ONLY VERSION OF THIS FILE. G3 HAS ITS OWN.
+
+This file contains flags for the C++ compiler, referred to by Bazel as copts.
+
+This is a new file
+"""
diff --git a/bazel/defines.bzl b/bazel/defines.bzl
deleted file mode 100644
index d7d17e863c..0000000000
--- a/bazel/defines.bzl
+++ /dev/null
@@ -1,9 +0,0 @@
-"""
-THIS IS THE EXTERNAL-ONLY VERSION OF THIS FILE. G3 HAS ITS OWN.
-
-This file contains customizable C++ defines.
-
-The file was deleted
-"""
-
-EXTRA_DEFINES = []  # This should always be empty externally. Add new defines in //bazel/BUILD.bazel
diff --git a/src/pathops/SkPathOpsDebug.h b/src/pathops/SkPathOpsDebug.h
index f3265464e3..9ef1be7dba 100644
--- a/src/pathops/SkPathOpsDebug.h
+++ b/src/pathops/SkPathOpsDebug.h
@@ -12,0 +13,3 @@
+#include "src/core/SkGeometry.h"
+#include "src/pathops/SkPathOpsPoint.h"
+#include "src/pathops/SkPathOpsTypes.h"
@@ -25,5 +27,0 @@ class SkPath;
-struct SkDConic;
-struct SkDCubic;
-struct SkDLine;
-struct SkDPoint;
-struct SkDQuad;
@@ -62 +60 @@ enum class SkOpPhase : char;
-#if FORCE_RELEASE
+#if FORCE_RELEASE_FOR_REAL
@@ -351,3 +349,3 @@ SkOpContour* AngleContour(SkOpAngle*, int id);
-const SkOpPtT* AnglePtT(const SkOpAngle*, int id);
-const SkOpSegment* AngleSegment(const SkOpAngle*, int id);
-const SkOpSpanBase* AngleSpan(const SkOpAngle*, int id);
+const SkOpPtT* AnglePtT2(const SkOpAngle*, int id);
+const SkOpSegment* AngleSegment3(const SkOpAngle*, int id);
+const SkOpSpanBase* AngleSpan4(const SkOpAngle*, int id);
`
	changedFiles, deletedFiles := extractChangedAndDeletedFiles(input)
	assert.Equal(t, deletedFiles, []string{"bazel/defines.bzl"})
	assert.Equal(t, changedFiles, []fileWithChanges{
		{
			fileName: "bazel/copts2.bzl",
			touchedLines: []lineOfCode{{
				contents: `"""`,
				num:      1,
			}, {
				contents: `THIS IS THE EXTERNAL-ONLY VERSION OF THIS FILE. G3 HAS ITS OWN.`,
				num:      2,
			}, {
				contents: ``,
				num:      3,
			}, {
				contents: `This file contains flags for the C++ compiler, referred to by Bazel as copts.`,
				num:      4,
			}, {
				contents: ``,
				num:      5,
			}, {
				contents: `This is a new file`,
				num:      6,
			}, {
				contents: `"""`,
				num:      7,
			}},
		},
		{
			fileName: "src/pathops/SkPathOpsDebug.h",
			touchedLines: []lineOfCode{{
				contents: `#include "src/core/SkGeometry.h"`,
				num:      13,
			}, {
				contents: `#include "src/pathops/SkPathOpsPoint.h"`,
				num:      14,
			}, {
				contents: `#include "src/pathops/SkPathOpsTypes.h"`,
				num:      15,
			}, {
				contents: `#if FORCE_RELEASE_FOR_REAL`,
				num:      60,
			}, {
				contents: `const SkOpPtT* AnglePtT2(const SkOpAngle*, int id);`,
				num:      349,
			}, {
				contents: `const SkOpSegment* AngleSegment3(const SkOpAngle*, int id);`,
				num:      350,
			}, {
				contents: `const SkOpSpanBase* AngleSpan4(const SkOpAngle*, int id);`,
				num:      351,
			}},
		},
	})
}

func TestExtractChangedAndDeletedFiles_WhitespaceChange_WholeLineCaptured(t *testing.T) {
	const input = `:100644 100644 464e32a3b8bc1f4e4238b6c089926d35febf3265 0000000000000000000000000000000000000000 M	bulk-triage-sk.ts

diff --git a/golden/modules/bulk-triage-sk/bulk-triage-sk.ts b/golden/modules/bulk-triage-sk/bulk-triage-sk.ts
index 7f7612c28..aa1d04715 100644
--- a/golden/modules/bulk-triage-sk/bulk-triage-sk.ts
+++ b/golden/modules/bulk-triage-sk/bulk-triage-sk.ts
@@ -142 +142 @@ export class BulkTriageSk extends ElementSk {
-  private onToggleAllCheckboxClick(e: Event) {
+	private onToggleAllCheckboxClick(e: Event) {
@@ -148 +148 @@ export class BulkTriageSk extends ElementSk {
-  private onCancelBtnClick() {
+	private onCancelBtnClick() {
`
	changedFiles, deletedFiles := extractChangedAndDeletedFiles(input)
	assert.Empty(t, deletedFiles)
	assert.Equal(t, changedFiles, []fileWithChanges{
		{
			fileName: "golden/modules/bulk-triage-sk/bulk-triage-sk.ts",
			touchedLines: []lineOfCode{{
				contents: "\tprivate onToggleAllCheckboxClick(e: Event) {",
				num:      142,
			}, {
				contents: "\tprivate onCancelBtnClick() {",
				num:      148,
			}},
		},
	})
}

func TestCheckLongLines(t *testing.T) {
	test := func(name string, input []fileWithChanges, expectedReturn bool, expectedLogs string) {
		t.Run(name, func(t *testing.T) {
			ctx, logs := captureLogs()
			ok := checkLongLines(ctx, input)
			assert.Equal(t, expectedReturn, ok)
			assert.Equal(t, expectedLogs, logs.String())
		})
	}

	test("short lines ok", []fileWithChanges{
		{
			fileName: "file1.txt",
			touchedLines: []lineOfCode{{
				contents: `This line is plenty short`,
				num:      281,
			}, {
				contents: `As is this one`,
				num:      282,
			}},
		},
	}, true, "")

	test("Skipped files ok", []fileWithChanges{
		{
			fileName: "file1.go",
			touchedLines: []lineOfCode{{
				contents: `go files can have any length 12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890`,
				num:      2,
			}},
		},
		{
			fileName: "package-lock.json",
			touchedLines: []lineOfCode{{
				contents: `and a few specific files 12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890`,
				num:      7,
			}},
		},
	}, true, "")

	test("long lines are bad", []fileWithChanges{
		{
			fileName: "file1.txt",
			touchedLines: []lineOfCode{{
				contents: `This is a long line 12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890`,
				num:      281,
			}, {
				contents: `not this one`,
				num:      282,
			}},
		},
		{
			fileName: "file2.md",
			touchedLines: []lineOfCode{{
				contents: `short`,
				num:      123,
			}, {
				contents: `very long 12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890`,
				num:      456,
			}, {
				contents: `Also very long 12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890`,
				num:      459,
			}},
		},
	}, false, `file1.txt:281 Line too long (130/100)
file2.md:456 Line too long (120/100)
file2.md:459 Line too long (125/100)
`)
}

func TestCheckTODOHasOwner(t *testing.T) {
	test := func(name string, input []fileWithChanges, expectedReturn bool, expectedLogs string) {
		t.Run(name, func(t *testing.T) {
			ctx, logs := captureLogs()
			ok := checkTODOHasOwner(ctx, input)
			assert.Equal(t, expectedReturn, ok)
			assert.Equal(t, expectedLogs, logs.String())
		})
	}

	test("TODO with owner or bug is fine", []fileWithChanges{
		{
			fileName: "file1.go",
			touchedLines: []lineOfCode{{
				contents: `TODO(name)`,
				num:      2,
			}},
		},
		{
			fileName: "README.md",
			touchedLines: []lineOfCode{{
				contents: `TODO(skbug.com/13690)`,
				num:      7,
			}},
		},
	}, true, "")

	test("TODO without owner or bug is bad", []fileWithChanges{
		{
			fileName: "file1.go",
			touchedLines: []lineOfCode{{
				contents: `TODO whoops`,
				num:      2,
			}, {
				contents: `TODO should keep going`,
				num:      3,
			}},
		},
		{
			fileName: "README.md",
			touchedLines: []lineOfCode{{
				contents: `TODO (space before paren not ok)`,
				num:      7,
			}, {
				contents: `# Trailing TODO`,
				num:      8,
			}},
		},
	}, false, `file1.go:2 TODO without owner or bug
file1.go:3 TODO without owner or bug
README.md:7 TODO without owner or bug
README.md:8 TODO without owner or bug
`)
}

func TestCheckForStrayWhitespace(t *testing.T) {
	test := func(name string, input []fileWithChanges, expectedReturn bool, expectedLogs string) {
		t.Run(name, func(t *testing.T) {
			ctx, logs := captureLogs()
			ok := checkForStrayWhitespace(ctx, input)
			assert.Equal(t, expectedReturn, ok)
			assert.Equal(t, expectedLogs, logs.String())
		})
	}

	test("leading spaces and tabs are fine", []fileWithChanges{
		{
			fileName: "file1.go",
			touchedLines: []lineOfCode{{
				contents: "\t// Everything    is fine",
				num:      2,
			}},
		},
		{
			fileName: "README.md",
			touchedLines: []lineOfCode{{
				contents: "      Totally fine.",
				num:      7,
			}},
		},
	}, true, "")

	test("trailing space or tabs is bad", []fileWithChanges{
		{
			fileName: "file1.go",
			touchedLines: []lineOfCode{{
				contents: "Trailing spaces     ",
				num:      2,
			}, {
				contents: "Trailing tab\t",
				num:      3,
			}},
		},
		{
			fileName: "README.md",
			touchedLines: []lineOfCode{{
				// all spaces
				contents: "     ",
				num:      7,
			}},
		},
	}, false, `file1.go:2 Trailing whitespace
file1.go:3 Trailing whitespace
README.md:7 Trailing whitespace
`)
}

func TestCheckPythonFilesHaveNoTabs(t *testing.T) {
	test := func(name string, input []fileWithChanges, expectedReturn bool, expectedLogs string) {
		t.Run(name, func(t *testing.T) {
			ctx, logs := captureLogs()
			ok := checkPythonFilesHaveNoTabs(ctx, input)
			assert.Equal(t, expectedReturn, ok)
			assert.Equal(t, expectedLogs, logs.String())
		})
	}

	test("no tabs means no problem", []fileWithChanges{
		{
			fileName: "file.py",
			touchedLines: []lineOfCode{{
				contents: `  # leading spaces ok for Python`,
				num:      2,
			}, {
				contents: `print('\t escaped tabs ok also')`,
				num:      4,
			}},
		},
		{
			fileName: "README.txt",
			touchedLines: []lineOfCode{{
				contents: `  And text files`,
				num:      7,
			}},
		},
	}, true, "")

	test("Tabs ok in non-Python files", []fileWithChanges{
		{
			fileName: "file.go",
			touchedLines: []lineOfCode{{
				contents: "\t\t// leading tabs are fine",
				num:      2,
			}},
		},
		{
			fileName: "foo/bar/Makefile",
			touchedLines: []lineOfCode{{
				contents: "\tAlso ok",
				num:      7,
			}},
		},
		{
			fileName: "file.ts",
			touchedLines: []lineOfCode{{
				contents: "\t// We're cool with tabs on TypeScript files",
				num:      5,
			}},
		},
		{
			fileName: "foo/bar/README.md",
			touchedLines: []lineOfCode{{
				contents: "\tTabs are ok in markdown",
				num:      6,
			}},
		},
	}, true, "")

	test("Tabs not ok in Python files", []fileWithChanges{
		{
			fileName: "file.py",
			touchedLines: []lineOfCode{{
				contents: "\t# leading tabs no good",
				num:      2,
			}, {
				contents: "Middle tabs\t\tno good either",
				num:      5,
			}},
		},
	}, false, `file.py:2 Tab character not allowed
file.py:5 Tab character not allowed
`)
}

func TestCheckBannedGoAPIs(t *testing.T) {
	test := func(name string, input []fileWithChanges, expectedReturn bool, expectedLogs string) {
		t.Run(name, func(t *testing.T) {
			ctx, logs := captureLogs()
			ok := checkBannedGoAPIs(ctx, input)
			assert.Equal(t, expectedReturn, ok)
			assert.Equal(t, expectedLogs, logs.String())
		})
	}

	test("go files with ok APIs", []fileWithChanges{
		{
			fileName: "file.go",
			touchedLines: []lineOfCode{{
				contents: "foo := httputils.NewTimeoutClient()",
				num:      2,
			}, {
				contents: "fmt.Println(foo)",
				num:      3,
			}},
		},
	}, true, "")

	test("Non-go files with bad APIs", []fileWithChanges{
		{
			fileName: "file.txt",
			touchedLines: []lineOfCode{{
				contents: "reflect.DeepEqual()",
				num:      2,
			}, {
				contents: "os.Interrupt()",
				num:      3,
			}},
		},
	}, true, "")

	test("Go files with bad APIs", []fileWithChanges{
		{
			fileName: "file.go",
			touchedLines: []lineOfCode{{
				contents: "reflect.DeepEqual()",
				num:      2,
			}, {
				contents: "os.Interrupt()",
				num:      3,
			}},
		},
		{
			fileName: "other.go",
			touchedLines: []lineOfCode{{
				contents: `exec.CommandContext(ctx, "git", "diff")`,
				num:      14,
			}},
		},
	}, false, `file.go:2 Instead of reflect.DeepEqual, please use DeepEqual in go.skia.org/infra/go/testutils
file.go:3 Instead of os.Interrupt, please use AtExit in go.skia.org/go/cleanup
other.go:14 Instead of "git", please use Executable in go.skia.org/infra/go/git
`)
}

func TestCheckJSDebugging(t *testing.T) {
	test := func(name string, input []fileWithChanges, expectedReturn bool, expectedLogs string) {
		t.Run(name, func(t *testing.T) {
			ctx, logs := captureLogs()
			ok := checkJSDebugging(ctx, input)
			assert.Equal(t, expectedReturn, ok)
			assert.Equal(t, expectedLogs, logs.String())
		})
	}

	test("normal js and ts code ok", []fileWithChanges{
		{
			fileName: "file.js",
			touchedLines: []lineOfCode{{
				contents: "console.log('hello world')",
				num:      2,
			}, {
				contents: "it('runs a unit test', () => {",
				num:      3,
			}},
		},
		{
			fileName: "file.ts",
			touchedLines: []lineOfCode{{
				contents: "console.log('hello world')",
				num:      2,
			}, {
				contents: "it('runs a unit test', () => {",
				num:      3,
			}},
		},
	}, true, "")

	test("Non-js files with debug lines ok", []fileWithChanges{
		{
			fileName: "file.txt",
			touchedLines: []lineOfCode{{
				contents: "debugger;",
				num:      2,
			}, {
				contents: "it.only() is how you can make...",
				num:      3,
			}},
		},
	}, true, "")

	test("ts or js files with debug lines are bad", []fileWithChanges{
		{
			fileName: "file.js",
			touchedLines: []lineOfCode{{
				contents: "console.log('hello world')",
				num:      2,
			}, {
				contents: "debugger;",
				num:      3,
			}},
		},
		{
			fileName: "file.ts",
			touchedLines: []lineOfCode{{
				contents: "describe.only('something to test', () => {",
				num:      4,
			}, {
				contents: "it.only('runs a unit test', () => {",
				num:      5,
			}},
		},
	}, false, `file.js:3 debugging code found (debugger;)
file.ts:4 debugging code found (describe.only()
file.ts:5 debugging code found (it.only()
`)
}

func TestCheckNonASCII(t *testing.T) {
	test := func(name string, input []fileWithChanges, expectedReturn bool, expectedLogs string) {
		t.Run(name, func(t *testing.T) {
			ctx, logs := captureLogs()
			ok := checkNonASCII(ctx, input)
			assert.Equal(t, expectedReturn, ok)
			assert.Equal(t, expectedLogs, logs.String())
		})
	}

	test("ASCII text ok everywhere", []fileWithChanges{
		{
			fileName: "file.css",
			touchedLines: []lineOfCode{{
				contents: "normal everyday text",
				num:      2,
			}, {
				contents: "Symbols *$&@(!",
				num:      3,
			}},
		},
	}, true, "")

	test("go files with ASCII ok", []fileWithChanges{
		{
			fileName: "file.go",
			touchedLines: []lineOfCode{{
				contents: "⛄",
				num:      2,
			}, {
				contents: "UTF-8 allowed: ✔️",
				num:      3,
			}},
		},
	}, true, "")

	test("other files with non-ascii are not ok", []fileWithChanges{
		{
			fileName: "file.py",
			touchedLines: []lineOfCode{{
				contents: "⛄⛄⛄⛄",
				num:      2,
			}},
		},
		{
			fileName: "file.css",
			touchedLines: []lineOfCode{{
				contents: "This line is ok",
				num:      4,
			}, {
				contents: "✔✔✔",
				num:      5,
			}, {
				contents: "snowman: ⛄",
				num:      5,
			}},
		},
	}, false, `file.py:2:1 Non ASCII character found
file.css:5:1 Non ASCII character found
file.css:5:10 Non ASCII character found
`)
}

// captureLogs returns a context and a buffer, with the latter being added to the former such that
// all calls to logf will write into the buffer instead of, for example, stdout.
func captureLogs() (context.Context, *bytes.Buffer) {
	var buf bytes.Buffer
	ctx := withOutputWriter(context.Background(), &buf)
	return ctx, &buf
}
