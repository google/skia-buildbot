package du

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"go.skia.org/infra/go/skerr"
)

// File represents a single file and its size in bytes.
type File struct {
	Name string
	Size uint64
}

// Dir represents a directory and all of its files and subdirectories, along
// with their total counts and sizes.
type Dir struct {
	Name       string
	Dirs       []*Dir
	Files      []*File
	TotalSize  uint64
	TotalFiles uint64
}

// Usage produces a report of disk usage within the given directory.
func Usage(ctx context.Context, rootPath string) (*Dir, error) {
	if err := ctx.Err(); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Find all of the files and directories.
	var root *Dir
	dirsByPath := map[string]*Dir{}
	err := filepath.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			// Skip directories we don't have permission to read.
			if errors.Is(skerr.Unwrap(err), fs.ErrPermission) {
				return nil
			}
			return skerr.Wrap(err)
		}
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}

		if info.IsDir() {
			dirEntry := &Dir{
				Dirs:      []*Dir{},
				Files:     []*File{},
				TotalSize: uint64(info.Size()),
			}
			dirsByPath[path] = dirEntry
			if path == rootPath || path == "" {
				dirEntry.Name = rootPath
				root = dirEntry
			} else {
				parentPath, base := filepath.Split(path)
				if parentPath == "" {
					parentPath = rootPath
				}
				parentPath = strings.TrimRight(parentPath, "/")
				dirEntry.Name = base
				parent, ok := dirsByPath[parentPath]
				if !ok {
					return skerr.Fmt("no directory entry found for %q", parentPath)
				}
				parent.Dirs = append(parent.Dirs, dirEntry)
			}
			return nil
		}

		parentPath, base := filepath.Split(path)
		if parentPath == "" {
			parentPath = rootPath
		}
		parentPath = strings.TrimRight(parentPath, "/")
		parent, ok := dirsByPath[parentPath]
		if !ok {
			return skerr.Fmt("no directory entry found for %q", parentPath)
		}
		parent.Files = append(parent.Files, &File{
			Name: base,
			Size: uint64(info.Size()),
		})
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Produce summaries for total number of files and bytes for each directory
	// and sort files and dirs by size for cleaner reporting.
	var genSummaries func(dir *Dir)
	genSummaries = func(dir *Dir) {
		for _, subdir := range dir.Dirs {
			genSummaries(subdir)
			dir.TotalFiles += subdir.TotalFiles
			dir.TotalSize += subdir.TotalSize
		}
		for _, file := range dir.Files {
			dir.TotalFiles++
			dir.TotalSize += file.Size
		}
		sort.Sort(DirSlice(dir.Dirs))
		sort.Sort(FileSlice(dir.Files))
	}
	genSummaries(root)

	return root, nil
}

// GenerateReport generates a textual report of the disk usage in the given
// directory. maxDepth controls how many directory levels are displayed;
// if zero, there is no maximum depth. human causes the report to use human-
// readable units instead of raw bytes.
func GenerateReport(ctx context.Context, rootDir *Dir, rootPath string, maxDepth int, human bool) (string, error) {
	var buf strings.Builder
	var print func(dir *Dir, parentPath string, depth int) error
	print = func(dir *Dir, parentPath string, depth int) error {
		dirPath := filepath.Join(parentPath, dir.Name)
		if depth < maxDepth || maxDepth == 0 {
			for _, subdir := range dir.Dirs {
				if err := print(subdir, dirPath, depth+1); err != nil {
					return err
				}
			}
		}
		var err error
		if human {
			_, err = fmt.Fprintf(&buf, "%s\t%s\n", humanize.Bytes(uint64(dir.TotalSize)), dirPath)
		} else {
			_, err = fmt.Fprintf(&buf, "%d\t%s\n", dir.TotalSize, dirPath)
		}
		if err != nil {
			return err
		}
		return nil
	}

	if err := print(rootDir, rootPath, 0); err != nil {
		return "", skerr.Wrap(err)
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// PrintReport generates and prints a textual report of the disk usage in the
// given directory. maxDepth controls how many directory levels are displayed;
// if zero, there is no maximum depth. human causes the report to use human-
// readable units instead of raw bytes.
func PrintReport(ctx context.Context, rootPath string, maxDepth int, human bool) error {
	rootDir, err := Usage(ctx, rootPath)
	if err != nil {
		return skerr.Wrap(err)
	}
	report, err := GenerateReport(ctx, rootDir, rootPath, maxDepth, human)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(report)
	return nil
}

// GenerateJSONReport generates and prints a JSON report of the disk usage in
// the given directory. maxDepth controls how many directory levels are
// displayed; if zero, there is no maximum depth. human causes the report to use
// human-readable units instead of raw bytes.
func GenerateJSONReport(ctx context.Context, rootDir *Dir, rootPath string, maxDepth int, human bool) (string, error) {
	type node struct {
		Name string  `json:"name"`
		Dirs []*node `json:"dirs,omitempty"`
		Size string  `json:"size"`
	}

	var mkTree func(dir *Dir, parentPath string, depth int) *node
	mkTree = func(dir *Dir, parentPath string, depth int) *node {
		dirPath := filepath.Join(parentPath, dir.Name)
		n := &node{
			Name: dirPath,
		}
		if depth < maxDepth || maxDepth == 0 {
			for _, subdir := range dir.Dirs {
				n.Dirs = append(n.Dirs, mkTree(subdir, dirPath, depth+1))
			}
		}
		if human {
			n.Size = humanize.Bytes(uint64(dir.TotalSize))
		} else {
			n.Size = strconv.FormatUint(dir.TotalSize, 10)
		}
		return n
	}

	tree := mkTree(rootDir, rootPath, 0)
	b, err := json.Marshal(tree)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return string(b), nil
}

// PrintJSONReport generates and prints a JSON report of the disk usage in the
// given directory. maxDepth controls how many directory levels are displayed;
// if zero, there is no maximum depth. human causes the report to use human-
// readable units instead of raw bytes.
func PrintJSONReport(ctx context.Context, rootPath string, maxDepth int, human bool) error {
	rootDir, err := Usage(ctx, rootPath)
	if err != nil {
		return skerr.Wrap(err)
	}
	report, err := GenerateJSONReport(ctx, rootDir, rootPath, maxDepth, human)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(report)
	return nil
}

// DirSlice is used for sorting.
type DirSlice []*Dir

// Len implements sort.Interface.
func (s DirSlice) Len() int {
	return len(s)
}

// Less implements sort.Interface.
func (s DirSlice) Less(a, b int) bool {
	return s[a].TotalSize < s[b].TotalSize
}

// Swap implements sort.Interface.
func (s DirSlice) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

// FileSlice is used for sorting.
type FileSlice []*File

// Len implements sort.Interface.
func (s FileSlice) Len() int {
	return len(s)
}

// Less implements sort.Interface.
func (s FileSlice) Less(a, b int) bool {
	return s[a].Size < s[b].Size
}

// Swap implements sort.Interface.
func (s FileSlice) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}
