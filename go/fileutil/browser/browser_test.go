package browser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

const (
	ansiEnter     = "\n"
	ansiDown      = "\x1b[B"
	ansiUp        = "\x1b[A"
	ansiRight     = "\x1b[C"
	ansiLeft      = "\x1b[D"
	ansiBackspace = "\x7f"
	ansiQuit      = "\x03"
)

type testReader struct {
	chunks []string
	index  int
}

func (t *testReader) Read(p []byte) (n int, err error) {
	if t.index >= len(t.chunks) {
		return 0, io.EOF
	}
	chunk := t.chunks[t.index]
	t.index++
	copy(p, chunk)
	return len(chunk), nil
}

func (t *testReader) Fd() uintptr {
	return 0
}

type mockPrefixFS struct {
	MapFS fstest.MapFS
}

func (m *mockPrefixFS) Open(name string) (fs.File, error) {
	return m.MapFS.Open(name)
}

func (m *mockPrefixFS) ReadDirPrefix(prefix string, limit int) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	for name, file := range m.MapFS {
		if strings.HasPrefix(name, prefix) {
			// Extract base name relative to parent directory part
			dirPart := ""
			lastSlash := strings.LastIndex(prefix, "/")
			if lastSlash != -1 {
				dirPart = prefix[:lastSlash+1]
			}
			relName := strings.TrimPrefix(name, dirPart)

			// Check if it's a directory (contains a slash after relName)
			isDir := false
			if slashIdx := strings.Index(relName, "/"); slashIdx != -1 {
				relName = relName[:slashIdx]
				isDir = true
			}

			// Deduplicate directory/file entries
			alreadyExists := false
			for _, existing := range entries {
				if existing.Name() == relName {
					alreadyExists = true
					break
				}
			}
			if alreadyExists {
				continue
			}

			entries = append(entries, &mockDirEntry{
				name:    relName,
				isDir:   isDir,
				size:    int64(len(file.Data)),
				modTime: file.ModTime,
			})
		}
	}

	// Sort so output is lexicographically stable for the test filesystem
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

type mockDirEntry struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
}

func (m *mockDirEntry) Name() string { return m.name }
func (m *mockDirEntry) IsDir() bool  { return m.isDir }
func (m *mockDirEntry) Type() fs.FileMode {
	if m.isDir {
		return fs.ModeDir
	}
	return 0
}
func (m *mockDirEntry) Info() (fs.FileInfo, error) {
	return &mockFileInfo{name: m.name, size: m.size, modTime: m.modTime, isDir: m.isDir}, nil
}

type mockFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string { return m.name }
func (m *mockFileInfo) Size() int64  { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode {
	if m.isDir {
		return fs.ModeDir
	}
	return 0
}
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// cleanFrame strips raw ANSI redrawing and color prefixes so we can assert
// against clear, human-readable terminal lines of a single frame.
func cleanFrame(f string) string {
	f = strings.ReplaceAll(f, "\r\n", "\n")
	f = strings.ReplaceAll(f, "\r", "")
	// Clean selection colors
	f = strings.ReplaceAll(f, ansiBoldCyan, "")
	f = strings.ReplaceAll(f, ansiReset, "")
	// Clean search colors
	f = strings.ReplaceAll(f, ansiBoldYellow, "")
	// Clean relative screen clears and carriage returns
	f = strings.ReplaceAll(f, ansiClearDisplay, "")
	f = strings.ReplaceAll(f, ansiEraseLine, "")

	// Split into lines, trim trailing space, and collapse multiple columns to keep expected layout clean
	lines := strings.Split(f, "\n")
	reg := regexp.MustCompile(` {3,}`)
	for i, l := range lines {
		trimmed := strings.TrimRight(l, " \t")
		lines[i] = reg.ReplaceAllString(trimmed, " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// Setup common mock prefix filesystem with explicit modification times.
func setupMockFS() fs.FS {
	return &mockPrefixFS{
		MapFS: fstest.MapFS{
			"GenerateReport/skia/main/2026-07-21T14:30:10Z": &fstest.MapFile{Data: []byte("report2"), ModTime: time.Date(2026, 7, 21, 14, 30, 10, 0, time.UTC)},
			"GenerateReport/skia/main/2026-07-22T14:35:21Z": &fstest.MapFile{Data: []byte("report1"), ModTime: time.Date(2026, 7, 22, 14, 35, 21, 0, time.UTC)},
			"GetTaskSummary/4827401740921446":               &fstest.MapFile{Data: []byte("summary1"), ModTime: time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)},
			"GetTaskSummary/5103450917240832":               &fstest.MapFile{Data: []byte("summary2"), ModTime: time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)},
		},
	}
}

func TestBrowser_Traversal(t *testing.T) {
	ctx := context.Background()
	defaultFS := setupMockFS()

	runTest := func(name string, fsys fs.FS, inputs []string, expected string) {
		t.Run(name, func(t *testing.T) {
			if fsys == nil {
				fsys = defaultFS
			}
			b := &Browser{
				Fsys: fsys,
				In:   &testReader{chunks: inputs},
				Out:  io.Discard,
			}

			selected, err := b.Browse(ctx, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if selected != expected {
				t.Errorf("expected %q, got %q", expected, selected)
			}
		})
	}

	// Recursive Traversal:
	// - Down arrow to go to "GetTaskSummary/" (index 1 of root list under "GenerateReport/")
	// - Enter to open it
	// - On load, the newest file "5103450917240832" (ModTime 11:00) is sorted first and auto-focused at index 1
	// - Enter to select it
	runTest("RecursiveTraversal", nil, []string{ansiDown, ansiEnter, ansiEnter}, "GetTaskSummary/5103450917240832")

	// Live Typing Filter:
	// - Down arrow to go to "GetTaskSummary/"
	// - Enter to open it
	// - Type "510" (which filters matches to "5103450917240832")
	// - Enter to select it
	runTest("LiveTypingFilter", nil, []string{ansiDown, ansiEnter, "5", "1", "0", ansiEnter}, "GetTaskSummary/5103450917240832")

	// Exact Match GCS Query:
	// - Down arrow to go to "GetTaskSummary/"
	// - Enter to open it
	// - Type/paste full exact name "5103450917240832" as a single paste event
	// - Enter to select it
	runTest("ExactMatchGCSQuery", nil, []string{ansiDown, ansiEnter, "5103450917240832", ansiEnter}, "GetTaskSummary/5103450917240832")

	// Adaptive Date Alphanumeric Sorting:
	// - Since 2026-07-22... and 2026-07-21... look like dates, they should sort DESCENDING (newest first).
	// - Keys:
	//   - Enter to open "GenerateReport/" (first item in root)
	//   - Enter to open "skia/"
	//   - Enter to open "main/"
	//   - Enter to select the first listed file (should be 2026-07-22T14:35:21Z, the newest!)
	runTest("AdaptiveDateAlphanumericSorting", nil, []string{ansiEnter, ansiEnter, ansiEnter, ansiEnter}, "GenerateReport/skia/main/2026-07-22T14:35:21Z")

	// Mixed Alphanumeric Date Sorting Transitivity:
	// We sort dates above non-dates (based on a simple regex), and dates are
	// sorted most recent first, whereas non-dates are sorted in increasing
	// alphanumerical order.  Therefore, we expect the order to be:
	//   1. "2026-07-30/" (newest date first)
	//   2. "2026-07-10/" (older date second)
	//   3. "2026-07-2a/" (non-date)
	// - Navigate:
	//   - Down arrow to go to "2026-07-10/" (index 1)
	//   - Down arrow to go to "2026-07-2a/" (index 2)
	//   - Enter to open "2026-07-2a/"
	//   - Enter to select "file.txt"
	runTest("MixedAlphanumericDateSortingTransitivity", &mockPrefixFS{
		MapFS: fstest.MapFS{
			"2026-07-2a/file.txt": &fstest.MapFile{Data: []byte("a")},
			"2026-07-30/file.txt": &fstest.MapFile{Data: []byte("b")},
			"2026-07-10/file.txt": &fstest.MapFile{Data: []byte("c")},
		},
	}, []string{ansiDown, ansiDown, ansiEnter, ansiEnter}, "2026-07-2a/file.txt")

	// Standard FS No PrefixReader:
	// - Navigate:
	//   - Enter on "reports/" (only folder in root)
	//   - Enter on "skia/"
	//   - Enter on "main/"
	//   - Enter to select "file1.txt" (should sort first)
	runTest("StandardFSNoPrefixReader", fstest.MapFS{
		"reports/skia/main/file1.txt": &fstest.MapFile{Data: []byte("file1")},
		"reports/skia/main/file2.txt": &fstest.MapFile{Data: []byte("file2")},
	}, []string{ansiEnter, ansiEnter, ansiEnter, ansiEnter}, "reports/skia/main/file1.txt")

	// Empty Subdirectory:
	// - Navigate:
	//   - We start at root "" where "empty-folder/" and "other-file.txt" are listed.
	//   - Focus is on "empty-folder/" (index 0).
	//   - Enter to open "empty-folder/".
	//   - Now inside "empty-folder/", which contains no files. The only item is "..".
	//   - Focus is on ".." (index 0).
	//   - Enter on ".." to go back to root.
	//   - Now back at root. Focus is on "empty-folder/" (index 0).
	//   - Down arrow to go to "other-file.txt" (index 1).
	//   - Enter to select "other-file.txt".
	runTest("EmptySubdirectory", &mockPrefixFS{
		MapFS: fstest.MapFS{
			"empty-folder/":  &fstest.MapFile{Data: []byte("")},
			"other-file.txt": &fstest.MapFile{Data: []byte("other")},
		},
	}, []string{ansiEnter, ansiEnter, ansiDown, ansiEnter}, "other-file.txt")

	// Search No Matches In Subdirectory:
	// - Navigate:
	//   - We start at root. Focus is on "sub/" (index 0, since "sub" < "other").
	//   - Press Enter to open "sub/".
	//   - Focus is on "file.txt" (index 1) since we are inside a subdirectory.
	//   - Type "x". This filters out "file.txt", leaving only ".." (len(entries) is 1).
	//   - This triggers searchChanged. Because there's only ".." left, focus safely resets to ".." (index 0).
	//   - Press Enter to select "..", going back to root.
	//   - Now we are back at root. Focus is on "sub/" (index 0).
	//   - Press Down arrow to focus "other-file.txt" (index 1).
	//   - Press Enter to select "other-file.txt".
	runTest("SearchNoMatchesInSubdirectory", &mockPrefixFS{
		MapFS: fstest.MapFS{
			"sub/file.txt":   &fstest.MapFile{Data: []byte("content")},
			"other-file.txt": &fstest.MapFile{Data: []byte("other")},
		},
	}, []string{ansiEnter, "x", ansiEnter, ansiDown, ansiEnter}, "other-file.txt")

	// Up Arrow Wrap Around:
	// - We start at root. Focus is on "GenerateReport/" (index 0).
	// - Press Up Arrow (ansiUp). This wraps around to focus "GetTaskSummary/" (index 1).
	// - Enter to open "GetTaskSummary/".
	// - On load, newest file "5103450917240832" is auto-focused (index 1).
	// - Enter to select it.
	runTest("UpArrowWrapAround", nil, []string{ansiUp, ansiEnter, ansiEnter}, "GetTaskSummary/5103450917240832")

	// Next Page Navigation (using ansiRight):
	// - Create a filesystem with 16 files (file01.txt to file16.txt).
	// - Since pageSize is 15, Page 1 will contain files 01 to 15.
	// - Press Right Arrow (ansiRight) to go to Page 2 (which contains file16.txt).
	// - Press Enter to select file16.txt.
	fsys16 := fstest.MapFS{}
	for i := 1; i <= 16; i++ {
		name := fmt.Sprintf("file%02d.txt", i)
		// We want file16.txt to be the oldest (sort last, so it lands on Page 2)
		// and file01.txt to be the newest (sort first, so it lands on Page 1 at index 0)
		modTime := time.Date(2026, 7, 22, 12-i, 0, 0, 0, time.UTC)
		fsys16[name] = &fstest.MapFile{Data: []byte("content"), ModTime: modTime}
	}
	runTest("NextPageNavigation", fsys16, []string{ansiRight, ansiEnter}, "file16.txt")

	// Prev Page Navigation (using ansiLeft):
	// - Start on Page 1 (files 01 to 15).
	// - Press Right Arrow (ansiRight) to navigate to Page 2.
	// - Press Left Arrow (ansiLeft) to navigate back to Page 1.
	// - Press Enter to select file01.txt (which sorts first).
	runTest("PrevPageNavigation", fsys16, []string{ansiRight, ansiLeft, ansiEnter}, "file01.txt")
}

func TestBrowser_StateTransitionsAndRendering(t *testing.T) {
	ctx := context.Background()
	mockFS := setupMockFS()

	// Setup input keypress chunks to step through a full terminal browser cycle:
	// 1. Initial State: List top-level folders. Focus is on first folder ("GenerateReport/") by default.
	// 2. Down Arrow ("\x1b[B"): Moves focus cursor to "GetTaskSummary/".
	// 3. Enter ("\n"): Opens "GetTaskSummary/" subdirectory. Bypasses ".." parent selection and auto-focuses first content file ("5103450917240832").
	// 4. Down Arrow ("\x1b[B"): Moves focus cursor to older file ("4827401740921446").
	// 5. Backspace ("\x7f"): Clears subfolder and returns up to the parent directory root.
	// 6. Ctrl+C ("\x03"): Quits the interactive session cleanly.
	inputChunks := []string{
		ansiDown,
		ansiEnter,
		ansiDown,
		ansiBackspace,
		ansiQuit,
	}

	var outBuf bytes.Buffer
	b := &Browser{
		Fsys: mockFS,
		In:   &testReader{chunks: inputChunks},
		Out:  &outBuf,
	}

	selected, err := b.Browse(ctx, "")
	// We expect browser to quit with ErrUserCanceled since our last key was Ctrl+C.
	if err != ErrUserCanceled {
		t.Fatalf("expected ErrUserCanceled, got error: %v, selected: %q", err, selected)
	}

	// Split terminal output history by the relative-up redrawing escape sequence ("\x1b[19A")
	// to isolate and verify each individual frame written to the output writer.
	rawFrames := strings.Split(outBuf.String(), "\x1b[19A")

	var frames []string
	for _, f := range rawFrames {
		cleaned := cleanFrame(f)
		if cleaned != "" {
			frames = append(frames, cleaned)
		}
	}

	// We expect exactly 5 distinct visual states during this interactive session.
	expectedFramesCount := 5
	if len(frames) < expectedFramesCount {
		t.Fatalf("expected at least %d visual frames, got %d. Full output:\n%s", expectedFramesCount, len(frames), outBuf.String())
	}

	// Define expected visual frames as clean, exact multiline strings.
	// Stable vertical spacing of 15 empty rows is correctly maintained between items and navigations.
	const (
		expectedFrame1 = `--- Folder: [FS Root] (Page 1/1) ---
> GenerateReport
  GetTaskSummary













------------------------------------------------------------
Arrow Keys [Up/Down]: Navigate  [Left/Right]: Prev/Next Page
Search: <type to filter>█ [Enter]: Select  [Backspace]: Back/Delete  [Ctrl+C]: Quit`

		expectedFrame2 = `--- Folder: [FS Root] (Page 1/1) ---
  GenerateReport
> GetTaskSummary













------------------------------------------------------------
Arrow Keys [Up/Down]: Navigate  [Left/Right]: Prev/Next Page
Search: <type to filter>█ [Enter]: Select  [Backspace]: Back/Delete  [Ctrl+C]: Quit`

		expectedFrame3 = `--- Folder: GetTaskSummary/ (Page 1/1) ---
  ..
> 5103450917240832 (0.0 KB)
  4827401740921446 (0.0 KB)












------------------------------------------------------------
Arrow Keys [Up/Down]: Navigate  [Left/Right]: Prev/Next Page
Search: <type to filter>█ [Enter]: Select  [Backspace]: Back/Delete  [Ctrl+C]: Quit`

		expectedFrame4 = `--- Folder: GetTaskSummary/ (Page 1/1) ---
  ..
  5103450917240832 (0.0 KB)
> 4827401740921446 (0.0 KB)












------------------------------------------------------------
Arrow Keys [Up/Down]: Navigate  [Left/Right]: Prev/Next Page
Search: <type to filter>█ [Enter]: Select  [Backspace]: Back/Delete  [Ctrl+C]: Quit`

		expectedFrame5 = `--- Folder: [FS Root] (Page 1/1) ---
  GenerateReport
> GetTaskSummary













------------------------------------------------------------
Arrow Keys [Up/Down]: Navigate  [Left/Right]: Prev/Next Page
Search: <type to filter>█ [Enter]: Select  [Backspace]: Back/Delete  [Ctrl+C]: Quit`
	)

	expectedSequence := []string{expectedFrame1, expectedFrame2, expectedFrame3, expectedFrame4, expectedFrame5}

	for idx, expectedFrame := range expectedSequence {
		actualFrame := frames[idx]
		if actualFrame != expectedFrame {
			t.Errorf("Visual Frame %d mismatch!\n--- EXPECTED ---\n%s\n--- ACTUAL ---\n%s\n--- DIFF ---", idx+1, expectedFrame, actualFrame)
		}
	}
}
