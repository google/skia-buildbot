// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package archiver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/luci/luci-go/common/isolated"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/luci/luci-go/common/runtime/tracer"
)

type walkItem struct {
	fullPath string
	relPath  string
	info     os.FileInfo
	err      error
}

// walk() enumerates a directory tree synchronously and sends the items to
// channel c.
//
// blacklist is a list of globs of files to ignore. Each blacklist glob is
// relative to root.
func walk(root string, blacklist []string, c chan<- *walkItem) {
	// TODO(maruel): Walk() sorts the file names list, which is not needed here
	// and slows things down. Options:
	// #1 Use os.File.Readdir() directly. It's in the stdlib and works fine, but
	//    it's not the most efficient implementation. On posix it does a lstat()
	//    call, on Windows it does a Win32FileAttributeData.
	// #2 Use raw syscalls.
	//   - On POSIX, use syscall.ReadDirent(). See src/os/dir_unix.go.
	//   - On Windows, use syscall.FindFirstFile(), syscall.FindNextFile(),
	//     syscall.FindClose() directly. See src/os/file_windows.go. For odd
	//     reasons, Windows does not have a batched version to reduce the number
	//     of kernel calls. It's as if they didn't care about performance.
	//
	// In practice, #2 may not be needed, the performance of #1 may be good
	// enough relative to the other performance costs. This needs to be perf
	// tested at 100k+ files scale on Windows and OSX.
	//
	// TODO(maruel): Cache directory enumeration. In particular cases (Chromium),
	// the same directory may be enumerated multiple times. Caching the content
	// may be worth. This needs to be perf tested.

	total := 0
	end := tracer.Span(root, "walk:"+filepath.Base(root), nil)
	defer func() { end(tracer.Args{"root": root, "total": total}) }()
	// Check patterns upfront, so it has consistent behavior w.r.t. bad glob
	// patterns.
	for _, b := range blacklist {
		if _, err := filepath.Match(b, b); err != nil {
			c <- &walkItem{err: fmt.Errorf("bad blacklist pattern \"%s\"", b)}
			return
		}
	}
	if strings.HasSuffix(root, string(filepath.Separator)) {
		root = root[:len(root)-1]
	}
	rootLen := len(root) + 1
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		total++
		if err != nil {
			return fmt.Errorf("walk(%s): %s", p, err)
		}
		if len(p) <= rootLen {
			// Root directory.
			return nil
		}
		relPath := p[rootLen:]
		for _, b := range blacklist {
			matched, _ := filepath.Match(b, relPath)
			if !matched {
				// Also check at the base file name.
				matched, _ = filepath.Match(b, filepath.Base(relPath))
			}
			if matched {
				// Must not return io.SkipDir for file, filepath.walk() handles this
				// badly.
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if info.IsDir() {
			return nil
		}
		c <- &walkItem{fullPath: p, relPath: relPath, info: info}
		return nil
	})
	if err != nil {
		c <- &walkItem{err: err}
	}
}

// PushDirectory walks a directory at root and creates a .isolated file.
//
// It walks the directories synchronously, then returns a *Item to signal when
// the background work is completed. The Item is signaled once all files are
// hashed. In particular, the *Item is signaled before server side cache
// lookups and upload is completed. Use archiver.Close() to wait for
// completion.
//
// relDir is a relative directory to offset relative paths against in the
// generated .isolated file.
//
// blacklist is a list of globs of files to ignore.
func PushDirectory(a *Archiver, root string, relDir string, blacklist []string) *Item {
	total := 0
	end := tracer.Span(a, "PushDirectory", tracer.Args{"path": relDir, "root": root})
	defer func() { end(tracer.Args{"total": total}) }()
	c := make(chan *walkItem)
	go func() {
		walk(root, blacklist, c)
		close(c)
	}()

	displayName := filepath.Base(root) + ".isolated"
	i := isolated.Isolated{
		Algo:    "sha-1",
		Files:   map[string]isolated.File{},
		Version: isolated.IsolatedFormatVersion,
	}
	items := []*Item{}
	s := &Item{DisplayName: displayName}
	for item := range c {
		if s.Error() != nil {
			// Empty the queue.
			continue
		}
		if item.err != nil {
			s.SetErr(item.err)
			continue
		}
		total++
		if relDir != "" {
			item.relPath = filepath.Join(relDir, item.relPath)
		}
		mode := item.info.Mode()
		if mode&os.ModeSymlink == os.ModeSymlink {
			l, err := os.Readlink(item.fullPath)
			if err != nil {
				s.SetErr(fmt.Errorf("readlink(%s): %s", item.fullPath, err))
				continue
			}
			i.Files[item.relPath] = isolated.File{Link: newString(l)}
		} else {
			i.Files[item.relPath] = isolated.File{
				Mode: newInt(int(mode.Perm())),
				Size: newInt64(item.info.Size()),
			}
			items = append(items, a.PushFile(item.relPath, item.fullPath, -item.info.Size()))
		}
	}
	if s.Error() != nil {
		return s
	}
	log.Printf("PushDirectory(%s) = %d files", root, len(i.Files))

	// Hashing, cache lookups and upload is done asynchronously.
	s.wgHashed.Add(1)
	go func() {
		defer s.wgHashed.Done()
		var err error
		for _, item := range items {
			item.WaitForHashed()
			if err = item.Error(); err != nil {
				break
			}
			name := item.DisplayName
			d := i.Files[name]
			d.Digest = item.Digest()
			i.Files[name] = d
		}
		if err == nil {
			raw := &bytes.Buffer{}
			if err = json.NewEncoder(raw).Encode(i); err == nil {
				if f := a.Push(displayName, isolatedclient.NewBytesSource(raw.Bytes()), 0); f != nil {
					f.WaitForHashed()
					if err = f.Error(); err == nil {
						s.lock.Lock()
						s.digestItem.Digest = string(f.Digest())
						s.lock.Unlock()
					}
				}
			}
		}
		if err != nil {
			s.SetErr(err)
		}
	}()
	return s
}
