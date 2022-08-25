package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	oneCLWithManyCommits = `commit d74307d70d7a7670e196aa6170eb6c16e8772845
HEAD -> remove-recipes$|2d1539110
commit 2d1539110acea5f9bb8e9e1099f204d16db3191e
$|4596653b1
commit 4596653b11cbb178b9ca43b6b7c9623213b395ba
$|8c92f96f7
commit 8c92f96f7f3924b9bf5088ad30c9b080882f1a85
$|f28ae292a
commit f28ae292af4ab27469ae569926d21fc4700ac8b3
$|aaa621543
commit aaa62154320fa246bd207b9435f4c1af271167f1
$|c1dd829c6
commit c1dd829c66fd2975fdd8ca7e6e4e2dd17dbc7600
$|98e2e2bfd
commit 98e2e2bfd6f1630b6a7d747950c336170699ff52
$|61f657f23
commit 61f657f2326f816273af80c089b5cbf2a5c29ea3
$|810ad2b89`
	oneCLWithOneCommit = `commit d74307d70d7a7670e196aa6170eb6c16e8772845
HEAD -> remove-recipes$|2d1539110`
	chainOfThreeCLs = `commit 8ab2fc2e7372c06fba206f4eff93b8ace5b170fe
HEAD -> try-colors$|d5332f245
commit d5332f24515f5734e50b7bbe7d138488de3d268d
$|827c99602 7a115508c
commit 7a115508c2e37fa82115ffd6ebae972295ef1f8b
tooltip$|97602c804
commit 97602c804391fbc5072486715064fa0727a46a7c
$|5f3a210e2 d9151b2bd
commit d9151b2bd16f717f450343a9589d49e96dd5e661
shorten-names-in-treemap$|50537bbb4
commit 50537bbb4ec4a803014627fc9fabc531ae13ca6d
$|c1b4ac60e
commit 827c996021c5983912b4f2552e3d71fd7293286a
$|5f3a210e2
commit 5f3a210e27d050c116aaecdc5d5db210c07373f0
$|9c4c078f6
commit 9c4c078f6636d7653c6eeb0473a630c1ad930b79
$|c1b4ac60e
commit c1b4ac60ec53e5890a89f109342f0f456f79efc0
$|9e2c5620c
commit 9e2c5620cca1f09962f866807e2a9b863ac15877
$|664b58284
commit 664b582843b7de27a1be6b2d02e03be2dc863161
$|d144eba2e`
)

func TestParseRevList_Success(t *testing.T) {
	test := func(name, input, expectedOutput string) {
		t.Run(name, func(t *testing.T) {
			actualOutput := parseRevList(input)
			assert.Equal(t, expectedOutput, actualOutput)
		})
	}

	test("one CL returns last parent", oneCLWithManyCommits, "810ad2b89")
	test("CL with one commit returns parent", oneCLWithOneCommit, "2d1539110")
	test("chain of CLs returns nearest branch commit", chainOfThreeCLs, "7a115508c2e37fa82115ffd6ebae972295ef1f8b")
	test("empty output means empty return value", "", "")
}

func TestParseGitDiff_NewDeletedModifiedFiles_AttributesLineNumbers(t *testing.T) {
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
	changedFiles, deletedFiles := parseGitDiff(input)
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

func TestParseGitDiff_WhitespaceChange_WholeLineCaptured(t *testing.T) {
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
	changedFiles, deletedFiles := parseGitDiff(input)
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

func TestCheckHasNoTabs(t *testing.T) {
	test := func(name string, input []fileWithChanges, expectedReturn bool, expectedLogs string) {
		t.Run(name, func(t *testing.T) {
			ctx, logs := captureLogs()
			ok := checkHasNoTabs(ctx, input)
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

	test("Tabs ok in go or Makefiles", []fileWithChanges{
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
	}, true, "")

	test("Tabs not ok in 'normal' files", []fileWithChanges{
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
		{
			fileName: "foo/bar/README.md",
			touchedLines: []lineOfCode{{
				contents: "\tNot ok in markdown",
				num:      7,
			}},
		},
	}, false, `file.py:2 Tab character not allowed
file.py:5 Tab character not allowed
foo/bar/README.md:7 Tab character not allowed
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
