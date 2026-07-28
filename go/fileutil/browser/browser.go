package browser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	// ANSI escape sequences for terminal manipulation.
	ansiReset        = "\x1b[0m"
	ansiBoldCyan     = "\x1b[1;36m"
	ansiBoldYellow   = "\x1b[1;33m"
	ansiEraseLine    = "\x1b[K"             // Erase from cursor to end of line
	ansiClearLine    = "\r" + ansiEraseLine // Return cursor and erase entire line
	ansiClearDisplay = "\x1b[J"             // Erase from cursor to end of screen
	ansiCursorUpFmt  = "\x1b[%dA"           // Format string to move cursor up %d lines
)

// FdReader combines io.Reader with an Fd() method, which is implemented
// by standard OS files like os.Stdin.
type FdReader interface {
	io.Reader
	Fd() uintptr
}

// PrefixReader is an optional interface that fs.FS can be implemented to
// improve efficiency for on-the-fly limited prefix directory searches.
// This is crucial for GCS, is a flat filesystem where directory structures are
// simulated, to avoid listing millions of files every time the user presses a
// key.
//
// ReadDirPrefix performs a prefix-based search, finding all immediate
// child files and subdirectories that match the specified prefix.
//
// For example, given a GCS bucket or filesystem containing:
//   - "foo/bar-file.txt"
//   - "foo/bar-sub/nested.txt"
//   - "foo/baz.txt"
//
// Calling ReadDirPrefix("foo/bar", 10) should return:
//  1. An fs.DirEntry representing the file:
//     - Name():    "bar-file.txt"
//     - IsDir():   false
//  2. An fs.DirEntry representing the subfolder:
//     - Name():    "bar-sub"
//     - IsDir():   true
//
// The file "foo/baz.txt" is omitted because it does not match the prefix, and
// "foo/bar-sub/nested.txt" is omitted because it is not an immediate child of
// "foo/".
type PrefixReader interface {
	ReadDirPrefix(prefix string, limit int) ([]fs.DirEntry, error)
}

// Browser encapsulates the state of the interactive directory explorer.
type Browser struct {
	Fsys fs.FS
	In   FdReader
	Out  io.Writer
}

// New creates a new Browser instance configured with os.Stdin and os.Stdout.
func New(fsys fs.FS) *Browser {
	return &Browser{
		Fsys: fsys,
		In:   os.Stdin,
		Out:  os.Stdout,
	}
}

// Browse launches a dynamic, interactive terminal-based directory browser
// starting at the given startPrefix over the filesystem. It returns the
// selected object's full path name or ErrUserCanceled if canceled.
func Browse(ctx context.Context, fsys fs.FS, startPrefix string) (string, error) {
	return New(fsys).Browse(ctx, startPrefix)
}

// Browse launches a dynamic, interactive terminal-based directory browser
// starting at the given startPrefix over the filesystem. It returns the
// selected object's full path name or ErrUserCanceled if canceled.
func (b *Browser) Browse(ctx context.Context, startPrefix string) (string, error) {
	return b.browsePrefixInteractive(ctx, startPrefix)
}

// ErrUserCanceled is returned if the user canceled browsing.
var ErrUserCanceled = errors.New("user canceled browsing")

func makeRaw(fd int) (*term.State, error) {
	if !term.IsTerminal(fd) {
		return nil, nil
	}
	return term.MakeRaw(fd)
}

func restore(fd int, state *term.State) error {
	if state == nil {
		return nil
	}
	return term.Restore(fd, state)
}

type keyType int

const (
	keyUnknown keyType = iota
	keyUp
	keyDown
	keyLeft
	keyRight
	keyEnter
	keyTab
	keyQuit
	keyBackspace
)

type browseEntry struct {
	Name        string
	DisplayName string
	IsDir       bool
	Size        int64
	Updated     time.Time
}

func (b *Browser) readKey() (keyType, rune, string, error) {
	var buf [256]byte
	n, err := b.In.Read(buf[:])
	if err != nil {
		return keyUnknown, 0, "", err
	}
	if n == 0 {
		return keyUnknown, 0, "", nil
	}

	if n > 1 {
		// Check if it's an arrow key escape sequence (3 bytes)
		if n == 3 && buf[0] == 27 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return keyUp, 0, "", nil
			case 'B':
				return keyDown, 0, "", nil
			case 'C':
				return keyRight, 0, "", nil
			case 'D':
				return keyLeft, 0, "", nil
			}
		}

		// Otherwise, it represents pasted text or quick successive key presses!
		var pasted strings.Builder
		for i := 0; i < n; i++ {
			b := buf[i]
			if b >= 32 && b <= 126 {
				pasted.WriteByte(b)
			}
		}
		pastedStr := pasted.String()
		if len(pastedStr) > 0 {
			return keyUnknown, 0, pastedStr, nil
		}
		return keyUnknown, 0, "", nil
	}

	raw := buf[0]
	switch raw {
	case 3: // Ctrl+C (ETX)
		return keyQuit, 0, "", nil
	case '\r', '\n':
		return keyEnter, 0, "", nil
	case '\t':
		return keyTab, 0, "", nil
	case 127, 8: // Backspace or Ctrl+H
		return keyBackspace, 0, "", nil
	}

	return keyUnknown, rune(raw), "", nil
}

func (b *Browser) fetchFolderEntries(prefix, subPrefix string, limit int, cache map[string]*browseEntry, cachedPrefixes map[string]bool) error {
	fullPrefix := prefix + subPrefix

	// 1. Check if we can short-circuit
	for cachedPref, fullyLoaded := range cachedPrefixes {
		if fullyLoaded {
			if strings.HasPrefix(subPrefix, cachedPref) {
				return nil
			}
		}
	}

	// 2. Check if we already queried this exact subPrefix
	if _, exists := cachedPrefixes[subPrefix]; exists {
		return nil
	}

	// 3. Query the filesystem
	var dirEntries []fs.DirEntry
	var err error
	fullyLoaded := true

	if pr, ok := b.Fsys.(PrefixReader); ok {
		dirEntries, err = pr.ReadDirPrefix(fullPrefix, limit)
		if err != nil {
			return err
		}
		if len(dirEntries) >= limit {
			fullyLoaded = false
		}
	} else {
		readPath := strings.TrimSuffix(prefix, "/")
		if readPath == "" {
			readPath = "."
		}
		dirEntries, err = fs.ReadDir(b.Fsys, readPath)
		if err != nil {
			return err
		}
	}

	// Process dirEntries into our cache
	for _, entry := range dirEntries {
		name := entry.Name()
		if prefix != "" {
			name = prefix + name
		}
		if entry.IsDir() && !strings.HasSuffix(name, "/") {
			name += "/"
		}

		if entry.Name() == ".." || entry.Name() == "" {
			continue
		}

		info, err := entry.Info()
		var size int64
		var updated time.Time
		if err == nil {
			size = info.Size()
			updated = info.ModTime()
		}

		cache[name] = &browseEntry{
			Name:        name,
			DisplayName: entry.Name(),
			IsDir:       entry.IsDir(),
			Size:        size,
			Updated:     updated,
		}
	}

	cachedPrefixes[subPrefix] = fullyLoaded
	return nil
}

var dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

func looksLikeDate(s string) bool {
	return dateRegex.MatchString(s)
}

func (b *Browser) browsePrefixInteractive(ctx context.Context, prefix string) (string, error) {
	cache := make(map[string]*browseEntry)
	cachedPrefixes := make(map[string]bool)

	err := b.fetchFolderEntries(prefix, "", 150, cache, cachedPrefixes)
	if err != nil {
		return "", err
	}

	if prefix == "" && len(cache) == 0 {
		return "", nil
	}

	pageSize := 15
	page := 0
	selectedOnPage := 0
	if prefix != "" {
		selectedOnPage = 1
	}

	searchBuf := ""

	// Put terminal in raw mode if Out is configured
	var oldState *term.State
	var fd int
	fd = int(b.In.Fd())
	oldState, err = makeRaw(fd)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = restore(fd, oldState)
	}()

	cleanup := func() {
		_ = restore(fd, oldState)
		// Move the cursor up by (pageSize + 4) lines and wipe everything from
		// the cursor to the end of the screen to cleanly clear the browser menu.
		_, _ = fmt.Fprintf(b.Out, ansiCursorUpFmt+ansiClearDisplay, pageSize+4)
	}

	firstDraw := true
	var numLines int

	for {
		// Build and filter active list of entries from our cache
		var entries []*browseEntry
		for _, entry := range cache {
			if entry.DisplayName == ".." {
				continue
			}
			if strings.HasPrefix(strings.ToLower(entry.DisplayName), strings.ToLower(searchBuf)) {
				entries = append(entries, entry)
			}
		}

		// Sort entries: Directories first, then Files
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir && !entries[j].IsDir {
				return true
			}
			if !entries[i].IsDir && entries[j].IsDir {
				return false
			}
			if entries[i].IsDir && entries[j].IsDir {
				iDate := looksLikeDate(entries[i].DisplayName)
				jDate := looksLikeDate(entries[j].DisplayName)
				if iDate && jDate {
					return entries[i].DisplayName > entries[j].DisplayName
				}
				if iDate && !jDate {
					return true // Dates come before non-dates
				}
				if !iDate && jDate {
					return false // Non-dates come after dates
				}
				return entries[i].DisplayName < entries[j].DisplayName
			}
			return entries[i].Updated.After(entries[j].Updated)
		})

		// Prepend '..' parent directory if we are not at the root
		if prefix != "" {
			trimmed := strings.TrimSuffix(prefix, "/")
			lastSlash := strings.LastIndex(trimmed, "/")
			parentPrefix := ""
			if lastSlash != -1 {
				parentPrefix = trimmed[:lastSlash+1]
			}

			entries = append([]*browseEntry{{
				Name:        parentPrefix,
				DisplayName: "..",
				IsDir:       true,
			}}, entries...)
		}

		start := page * pageSize
		if start >= len(entries) {
			start = 0
			page = 0
		}
		end := start + pageSize
		if end > len(entries) {
			end = len(entries)
		}

		numItemsOnPage := end - start
		if selectedOnPage >= numItemsOnPage {
			selectedOnPage = numItemsOnPage - 1
		}
		if selectedOnPage < 0 {
			selectedOnPage = 0
		}

		if !firstDraw {
			_, _ = fmt.Fprintf(b.Out, ansiCursorUpFmt, numLines)
		} else {
			// Pre-allocate maximum possible vertical space once at start to avoid screen scrolls during loop
			maxMenuLines := pageSize + 4
			for i := 0; i < maxMenuLines; i++ {
				_, _ = fmt.Fprint(b.Out, "\r\n")
			}
			_, _ = fmt.Fprintf(b.Out, ansiCursorUpFmt, maxMenuLines)
			numLines = maxMenuLines
		}
		firstDraw = false

		// Print Header
		header := fmt.Sprintf("--- Folder: %s (Page %d/%d) ---", prefix, page+1, (len(entries)+pageSize-1)/pageSize)
		if prefix == "" {
			header = fmt.Sprintf("--- Folder: [FS Root] (Page %d/%d) ---", page+1, (len(entries)+pageSize-1)/pageSize)
		}
		_, _ = fmt.Fprintf(b.Out, ansiClearLine+"%s\r\n", header)

		// Print Items
		for i := start; i < end; i++ {
			entry := entries[i]
			idxOnPage := i - start

			prefixChar := "  "
			if idxOnPage == selectedOnPage {
				prefixChar = "> "
			}

			var lineText string
			if entry.IsDir {
				lineText = entry.DisplayName
			} else {
				sizeKB := float64(entry.Size) / 1024.0
				lineText = fmt.Sprintf("%-50s (%.1f KB)", entry.DisplayName, sizeKB)
			}

			if idxOnPage == selectedOnPage {
				_, _ = fmt.Fprintf(b.Out, ansiClearLine+"%s"+ansiBoldCyan+"%s"+ansiReset+"\r\n", prefixChar, lineText)
			} else {
				_, _ = fmt.Fprintf(b.Out, ansiClearLine+"%s%s\r\n", prefixChar, lineText)
			}
		}

		// Empty lines if the items on the page are fewer than the pageSize, to
		// keep visual consistency and heights stable.
		for i := numItemsOnPage; i < pageSize; i++ {
			_, _ = fmt.Fprint(b.Out, ansiClearLine+"\r\n")
		}

		// Print Footer and Navigation hints
		_, _ = fmt.Fprint(b.Out, ansiClearLine+"------------------------------------------------------------\r\n")
		navHint := "Arrow Keys [Up/Down]: Navigate  [Left/Right]: Prev/Next Page"
		_, _ = fmt.Fprintf(b.Out, ansiClearLine+"%s\r\n", navHint)

		searchPrompt := "<type to filter>"
		if searchBuf != "" {
			searchPrompt = searchBuf
		}
		_, _ = fmt.Fprintf(b.Out, ansiClearLine+"Search: "+ansiBoldYellow+"%s"+ansiReset+"█   [Enter]: Select  [Backspace]: Back/Delete  [Ctrl+C]: Quit\r\n", searchPrompt)

		key, char, pastedStr, err := b.readKey()
		if err != nil {
			return "", err
		}

		if key == keyQuit {
			cleanup()
			return "", ErrUserCanceled
		}

		searchChanged := false

		switch key {
		case keyUp:
			if selectedOnPage > 0 {
				selectedOnPage--
			} else {
				selectedOnPage = numItemsOnPage - 1 // Wrap around
			}
		case keyDown:
			if selectedOnPage < numItemsOnPage-1 {
				selectedOnPage++
			} else {
				selectedOnPage = 0 // Wrap around
			}
		case keyLeft:
			if page > 0 {
				page--
				selectedOnPage = 0
			}
		case keyRight:
			if end < len(entries) {
				page++
				selectedOnPage = 0
			}
		case keyBackspace:
			if len(searchBuf) > 0 {
				searchBuf = searchBuf[:len(searchBuf)-1]
				searchChanged = true
			} else {
				// Backspacing on an empty search acts as "Go Back"
				cleanup()
				return "", nil
			}
		case keyEnter, keyTab:
			selectedIdx := start + selectedOnPage
			if selectedIdx >= 0 && selectedIdx < len(entries) {
				entry := entries[selectedIdx]
				if entry.IsDir {
					cleanup()

					// If they select '..', go back up
					if entry.DisplayName == ".." {
						return "", nil
					}

					// Recursively call browser on the subfolder
					subObj, err := b.browsePrefixInteractive(ctx, entry.Name)
					if err != nil {
						return "", err
					}
					if subObj != "" {
						return subObj, nil // Bubble up the selected object path!
					}

					// If subfolder was cancelled, re-enter raw mode and repaint current folder
					oldState, err = makeRaw(fd)
					if err != nil {
						return "", err
					}
					firstDraw = true // Trigger full repaint and pre-allocation
				} else {
					cleanup()
					return entry.Name, nil
				}
			}
		default:
			if pastedStr != "" {
				searchBuf += pastedStr
				searchChanged = true
			} else if char >= 32 && char <= 126 {
				searchBuf += string(char)
				searchChanged = true
			}
		}

		if searchChanged {
			if searchBuf != "" {
				err := b.fetchFolderEntries(prefix, searchBuf, 150, cache, cachedPrefixes)
				if err != nil {
					return "", err
				}
			}
			page = 0
			selectedOnPage = 0
			if prefix != "" {
				selectedOnPage = 1
			}
		}

		numLines = pageSize + 4
	}
}
