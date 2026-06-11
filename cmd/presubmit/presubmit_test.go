package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func TestExtractBranchBaseCitc(t *testing.T) {
	test := func(name, input, expectedOutput string) {
		t.Run(name, func(t *testing.T) {
			ctx, _ := captureLogs()
			actualOutput := extractBranchBaseCitc(input, ctx)
			assert.Equal(t, expectedOutput, actualOutput)
		})
	}

	const noLocalCommits = `Repo root: skia/buildbot
@  @ me 2026-05-21 12:56:09 @
│  (no description set)
◆  1741c3e27daf me 2026-05-21 08:45:10 1741c3e2
│  Reland "[public's a subset] fetch logic"
◆  c41986669719 Wenbin Zhang 2026-05-16 01:36:47 c4198666
│  [perf] support summary bar interaction with the graph`
	test("no local commits returns immediate parent", noLocalCommits, "@-")

	const oneLocalCommit = `Repo root: skia/buildbot
@  @ me 2026-05-21 12:56:09 @
│  (no description set)
○  9d6d5a8c29c3 me 2026-05-21 12:46:55 9d6d5a8c
│  Make presubmit work from Cider-G
◆  1741c3e27daf me 2026-05-21 08:45:10 1741c3e2
│  Reland "[public's a subset] fetch logic"
◆  c41986669719 Wenbin Zhang 2026-05-16 01:36:47 c4198666
│  [perf] support summary bar interaction with the graph`
	test("one local commit returns grandparent", oneLocalCommit, "@--")

	const manyLocalCommits = `Repo root: skia/buildbot
@  @ me 2026-05-21 12:56:09 @
│  (no description set)
○  9d6d5a8c29c3 me 2026-05-21 12:46:55 9d6d5a8c
│  Commit 3
○  8d6d5a8c29c3 me 2026-05-21 12:46:50 8d6d5a8c
│  Commit 2
○  7d6d5a8c29c3 me 2026-05-21 12:46:40 7d6d5a8c
│  Commit 1
◆  1741c3e27daf me 2026-05-21 08:45:10 1741c3e2
│  Reland "[public's a subset] fetch logic"`
	test("multiple local commits returns remote base", manyLocalCommits, "@----")

	test("unrecognized format falls back to immediate parent", "unexpected format\n...", "@-")
}

func TestExtractChangedAndDeletedFilesCitc(t *testing.T) {
	const input = `Repo root: skia/buildbot
Diffing 1741c3e27dafd251ea8c1bd107d6b967cc5481af..@
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
@@ -348,4 +348,4 @@
-const SkOpPtT* AnglePtT(const SkOpAngle*, int id);
-const SkOpSegment* AngleSegment(const SkOpAngle*, int id);
-const SkOpSpanBase* AngleSpan(const SkOpAngle*, int id);
+const SkOpPtT* AnglePtT2(const SkOpAngle*, int id);
+const SkOpSegment* AngleSegment3(const SkOpAngle*, int id);
+const SkOpSpanBase* AngleSpan4(const SkOpAngle*, int id);
`
	changedFiles, deletedFiles := extractChangedAndDeletedFilesCitc(input)
	assert.Equal(t, []string{"bazel/defines.bzl"}, deletedFiles)
	assert.Equal(t, []fileWithChanges{
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
				num:      348,
			}, {
				contents: `const SkOpSegment* AngleSegment3(const SkOpAngle*, int id);`,
				num:      349,
			}, {
				contents: `const SkOpSpanBase* AngleSpan4(const SkOpAngle*, int id);`,
				num:      350,
			}},
		},
	}, changedFiles)
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
	}, false, `file.go:2 Instead of reflect.DeepEqual, please use Equal in go.skia.org/infra/go/deepequal/assertdeep
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

func TestRunAutoreview(t *testing.T) {
	t.Run("AI_PRESUBMIT_CHECK is not set", func(t *testing.T) {
		ctx, logs := captureLogs()
		ok := runAutoreview(ctx, "base-commit")
		assert.True(t, ok)
		assert.Equal(t, "Skip AI review.\n", logs.String())
	})

	t.Run("Autoreview successeds", func(t *testing.T) {
		t.Setenv("AI_PRESUBMIT_CHECK", "true")

		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		// The mock script writes its arguments to a file in the same directory.
		script := "#!/bin/sh\necho \"$@\" > \"$(dirname \"$0\")/args.txt\"\nexit 0\n"
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, _ := captureLogs()
		ok := runAutoreview(ctx, "base-commit")
		assert.True(t, ok)

		// Verify that the arguments were correct.
		argsFile := filepath.Join(tempDir, "args.txt")
		argsData, err := os.ReadFile(argsFile)
		assert.NoError(t, err)
		assert.Equal(
			t,
			"run --config=mayberemote //cmd/autoreview -- --base-commit=base-commit "+
				"--show-lgtm=false --show-warnings=false\n",
			string(argsData),
		)
	})

	t.Run("Autoreview fails", func(t *testing.T) {
		t.Setenv("AI_PRESUBMIT_CHECK", "true")

		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		err := os.WriteFile(
			mockBazelisk,
			[]byte("#!/bin/sh\necho 'some error'\nexit 1\n"),
			0755,
		)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, logs := captureLogs()
		ok := runAutoreview(ctx, "base-commit")
		assert.False(t, ok)
		assert.Equal(
			t,
			"some error\nautoreview failed or found a blocker!\n",
			logs.String(),
		)
	})
}

func TestRunGolangCILintForPinpoint(t *testing.T) {
	t.Run("No pinpoint go files - skips execution", func(t *testing.T) {
		ctx, _ := captureLogs()
		files := []fileWithChanges{
			{fileName: "perf/go/main.go"},
			{fileName: "pinpoint/README.md"},
		}

		ok := runGolangCILintForPinpoint(ctx, files, "HEAD~1", false)
		assert.True(t, ok)
	})

	t.Run("Pinpoint go files - success", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		script := "#!/bin/sh\necho \"$@\" > \"$(dirname \"$0\")/args.txt\"\nexit 0\n"
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, _ := captureLogs()
		files := []fileWithChanges{
			{fileName: "pinpoint/go/main.go"},
			{fileName: "perf/go/main.go"}, // Should be ignored
		}

		ok := runGolangCILintForPinpoint(ctx, files, "HEAD~1", false)
		assert.True(t, ok)

		// Verify arguments, ensuring perf/go/main.go was filtered out
		argsFile := filepath.Join(tempDir, "args.txt")
		argsData, err := os.ReadFile(argsFile)
		assert.NoError(t, err)
		expectedArgs := "run --config=mayberemote //:go -- run " + golangci +
			" run --new-from-rev HEAD~1 --whole-files ./pinpoint/go\n"
		assert.Equal(t, expectedArgs, string(argsData))
	})

	t.Run("With fix flag", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		script := "#!/bin/sh\necho \"$@\" > \"$(dirname \"$0\")/args.txt\"\nexit 0\n"
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, _ := captureLogs()
		files := []fileWithChanges{{fileName: "pinpoint/go/main.go"}}

		ok := runGolangCILintForPinpoint(ctx, files, "HEAD~1", true)
		assert.True(t, ok)

		argsFile := filepath.Join(tempDir, "args.txt")
		argsData, err := os.ReadFile(argsFile)
		assert.NoError(t, err)
		expectedArgs := "run --config=mayberemote //:go -- run " + golangci +
			" run --new-from-rev HEAD~1 --whole-files --fix ./pinpoint/go\n"
		assert.Equal(t, expectedArgs, string(argsData))
	})

	t.Run("Command fails", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		err := os.WriteFile(
			mockBazelisk,
			[]byte("#!/bin/sh\necho 'lint error'\nexit 1\n"),
			0755,
		)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, logs := captureLogs()
		files := []fileWithChanges{{fileName: "pinpoint/go/main.go"}}

		ok := runGolangCILintForPinpoint(ctx, files, "HEAD~1", false)
		assert.False(t, ok)
		assert.Contains(t, logs.String(), "lint error")
		assert.Contains(t, logs.String(), "golangci-lint failed!")
	})
}

func TestWorkflowCheckFormatPosn(t *testing.T) {
	repoRoot := "/path/to/repo"
	assert.Equal(t, "a/b.go:1:2", workflowCheckFormatPosn("/path/to/repo/a/b.go:1:2", repoRoot))
}

func TestWorkflowCheckFormatMessage(t *testing.T) {
	assert.Equal(t, "A\n-> B", workflowCheckFormatMessage("A -> B"))
}

func TestRunWorkflowCheck(t *testing.T) {
	t.Run("No files - skips execution", func(t *testing.T) {
		ctx, _ := captureLogs()
		ok := runWorkflowCheck(ctx, nil, "/path/to/repo")
		assert.True(t, ok)
	})

	t.Run("Success - no issues", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		script := "#!/bin/sh\necho '{}' >&1\nexit 0\n"
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, _ := captureLogs()
		files := []fileWithChanges{{fileName: "a/b.go"}}
		ok := runWorkflowCheck(ctx, files, "/path/to/repo")
		assert.True(t, ok)
	})

	t.Run("Failure - matching workflowcheck issues", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		stdoutJSON := `{
			"testpkg": {
				"workflowcheck": [
					{
						"posn": "/path/to/repo/a/b.go:1:2",
						"message": "some-determinism-issue"
					}
				]
			}
		}`
		script := fmt.Sprintf("#!/bin/sh\ncat << 'EOF'\n%s\nEOF\nexit 0\n", stdoutJSON)
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, logs := captureLogs()
		files := []fileWithChanges{{fileName: "a/b.go"}}
		ok := runWorkflowCheck(ctx, files, "/path/to/repo")
		assert.False(t, ok)

		assert.Contains(t, logs.String(), "workflowcheck: a/b.go:1:2: some-determinism-issue")
		assert.Contains(t, logs.String(), "workflowcheck: found 1 determinism issues.")
	})

	t.Run("Success - issues do not match changed files", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		stdoutJSON := `{
			"testpkg": {
				"workflowcheck": [
					{
						"posn": "/path/to/repo/a/c.go:1:2",
						"message": "some-determinism-issue"
					}
				]
			}
		}`
		script := fmt.Sprintf("#!/bin/sh\ncat << 'EOF'\n%s\nEOF\nexit 0\n", stdoutJSON)
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, logs := captureLogs()
		files := []fileWithChanges{{fileName: "a/b.go"}}
		ok := runWorkflowCheck(ctx, files, "/path/to/repo")
		assert.True(t, ok)
		assert.Empty(t, logs.String())
	})

	t.Run("Failure - command execution fails", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		script := "#!/bin/sh\necho 'bazel errors' >&2\nexit 1\n"
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, logs := captureLogs()
		files := []fileWithChanges{{fileName: "a/b.go"}}
		ok := runWorkflowCheck(ctx, files, "/path/to/repo")
		assert.False(t, ok)

		assert.Contains(t, logs.String(), "bazel errors")
		assert.Contains(t, logs.String(), "workflowcheck failed to run")
	})

	t.Run("Failure - invalid JSON", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		script := "#!/bin/sh\necho 'not-json-output'\nexit 0\n"
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, logs := captureLogs()
		files := []fileWithChanges{{fileName: "a/b.go"}}
		ok := runWorkflowCheck(ctx, files, "/path/to/repo")
		assert.False(t, ok)

		assert.Contains(t, logs.String(), "workflowcheck returned invalid JSON")
		assert.Contains(t, logs.String(), "not-json-output")
	})
}

func TestRunStylelint(t *testing.T) {
	t.Run("No scss files - skips execution", func(t *testing.T) {
		ctx, _ := captureLogs()
		files := []fileWithChanges{
			{fileName: "perf/go/main.go"},
			{fileName: "pinpoint/README.md"},
			{fileName: "app/test.ts"},
		}

		ok := runStylelint(ctx, files, "/path/to/repo", "HEAD~1")
		assert.True(t, ok)
	})

	t.Run("SCSS files - success", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		script := "#!/bin/sh\necho \"$@\" > \"$(dirname \"$0\")/args.txt\"\nexit 0\n"
		err := os.WriteFile(mockBazelisk, []byte(script), 0755)
		assert.NoError(t, err)

		mockGit := filepath.Join(tempDir, "git")
		const gitScript = `#!/bin/sh
if [ "$1" = "citc" ]; then
  cat << 'EOF'
--- a/app/styles.scss
+++ b/app/styles.scss
--- a/perf/go/main.go
+++ b/perf/go/main.go
EOF
else
  cat << 'EOF'
diff --git a/app/styles.scss b/app/styles.scss
diff --git a/perf/go/main.go b/perf/go/main.go
EOF
fi
exit 0
`
		err = os.WriteFile(mockGit, []byte(gitScript), 0755)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, _ := captureLogs()
		files := []fileWithChanges{
			{fileName: "app/styles.scss"},
			{fileName: "perf/go/main.go"}, // Should be ignored
		}

		ok := runStylelint(ctx, files, "/path/to/repo", "HEAD~1")
		assert.True(t, ok)

		// Verify arguments, ensuring perf/go/main.go was filtered out
		argsFile := filepath.Join(tempDir, "args.txt")
		argsData, err := os.ReadFile(argsFile)
		assert.NoError(t, err)
		expectedArgs := "run --config=mayberemote //:npx -- stylelint --quiet --fix app/styles.scss\n"
		assert.Equal(t, expectedArgs, string(argsData))
	})

	t.Run("Command fails", func(t *testing.T) {
		tempDir := t.TempDir()
		mockBazelisk := filepath.Join(tempDir, "bazelisk")
		err := os.WriteFile(
			mockBazelisk,
			[]byte("#!/bin/sh\necho 'stylelint error'\nexit 1\n"),
			0755,
		)
		assert.NoError(t, err)

		oldPath := os.Getenv("PATH")
		t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)

		ctx, logs := captureLogs()
		files := []fileWithChanges{{fileName: "app/styles.scss"}}

		ok := runStylelint(ctx, files, "/path/to/repo", "HEAD~1")
		assert.False(t, ok)
		assert.Contains(t, logs.String(), "stylelint error")
		assert.Contains(t, logs.String(), "stylelint failed!")
	})
}
