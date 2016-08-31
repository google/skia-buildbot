// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package managedfs offers a managed filesystem. A managed filesystem assumes
// that it has ownership of a specified directory and all of that directory's
// contents, and tracks directory and creation.
//
// This is useful in enabling the reuse of managed directories. In between uses,
// new files may be added and old files may be deleted. A managed filesystem
// allows the user to perform all of the creation operations and then, via
// "CleanUp", reduce the actual filesystem to the target state without
// performing a massive recursive deletion at the beginning.
//
// Additionally, the directory and file objects have utility functions that are
// useful to "deploytool".
package managedfs

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

// Filesystem is a managed directory struct. It will track all files and
// directories added to it.
//
// Its "CleanUp" method may be invoked, which will crawl the directory space
// and delete any directories and files that were not explicitly added during
// to it.
type Filesystem struct {
	// GenName is the name of the tool that will be referenced in generated
	// content.
	GenName string
	// GenHeader is generated header text lines that should be added to any
	// generated file.
	GenHeader []string

	// lock is a mutex that protects the contents of this Filesystem. It is shared
	// with all of its derivative Dir/File instances.
	lock sync.RWMutex

	// rootDir is the base managed directory.
	rootDir string

	// root is the root managed directory for this filesystem.
	root *Dir
}

// New creates a new managed filesystem rooted at the target directory.
//
// The filesystem will create rootDir if it does not exist, and assumes total
// ownership of its contents.
func New(rootDir string) (*Filesystem, error) {
	rootDir = filepath.Clean(rootDir)
	if err := ensureDirectory(rootDir); err != nil {
		return nil, errors.Annotate(err).Reason("failed to create root directory").Err()
	}
	fs := Filesystem{
		rootDir: rootDir,
	}
	fs.root = &Dir{
		fs:      &fs,
		relPath: "",
		elem:    "",
	}
	return &fs, nil
}

// Base returns the base Dir for this filesystem.
func (fs *Filesystem) Base() *Dir { return fs.root }

// Dir is a managed directory, rooted in a Filesystem.
type Dir struct {
	// fs is the parent filesystem.
	fs *Filesystem
	// parent is the parent directory
	parent *Dir

	// relPath is the directory's path relative to the the managed filesystem
	// root.
	relPath string
	// elem is the directory's specific element.
	elem string

	// ignored is true if this directory and its contents are ignored in cleanup.
	ignored bool

	subdirs map[string]*Dir
	files   map[string]struct{}
}

// String returns the full path of this managed directory.
func (d *Dir) String() string {
	if d.relPath == "" {
		return d.fs.rootDir
	}
	return filepath.Join(d.fs.rootDir, d.relPath)
}

// RelPath returns the path of this Dir relative to its filesystem root.
func (d *Dir) RelPath() string { return d.relPath }

// RelPathFrom returns the relative path from d to targpath. It also asserts
// that targpath is under the Filesystem root.
func (d *Dir) RelPathFrom(targpath string) (string, error) {
	if !isSubpath(d.fs.rootDir, targpath) {
		return "", errors.Reason("[%(path)s] is not a subpath of [%(root)s]").
			D("path", targpath).D("root", d.fs.rootDir).Err()
	}

	rel, err := filepath.Rel(d.String(), targpath)
	if err != nil {
		return "", errors.Annotate(err).Reason("could not calculate relative path from [%(dir)s] to [%(path)s]").
			D("dir", d.String()).D("path", targpath).Err()
	}
	return rel, nil
}

// Ignore configures d to be marked as ignored. This causes it and all of its
// contents to be exempt from cleanup.
func (d *Dir) Ignore() {
	d.fs.lock.Lock()
	defer d.fs.lock.Unlock()
	d.ignored = true
}

// GetSubDirectory returns a managed sub-directory composed by joining all of
// the specified elements.
//
// Unlike EnsureDirectory, this does not actually create the specified
// directory.
func (d *Dir) GetSubDirectory(elem string, elems ...string) *Dir {
	subDir := d.registerSubDir(elem)
	if len(elems) == 0 {
		return subDir
	}
	return subDir.GetSubDirectory(elems[0], elems[1:]...)
}

func (d *Dir) registerSubDir(elem string) (subDir *Dir) {
	// Optimistic: read lock to check if our subdirectory is already registered.
	d.fs.lock.RLock()
	subDir = d.subdirs[elem]
	d.fs.lock.RUnlock()

	if subDir != nil {
		return
	}

	// Doesn't exist. Take write lock, check again, and, if missing, register.
	d.fs.lock.Lock()
	defer d.fs.lock.Unlock()

	subDir = d.subdirs[elem]
	if subDir != nil {
		return
	}

	subDir = &Dir{
		fs:      d.fs,
		parent:  d,
		relPath: filepath.Join(d.relPath, elem),
		elem:    elem,
	}

	if d.subdirs == nil {
		d.subdirs = make(map[string]*Dir)
	}
	d.subdirs[elem] = subDir
	return
}

// EnsureDirectory returns a managed sub-directory composed by joining all of
// the specified elements together.
//
// If the described directory doesn't exist, it will be created.
func (d *Dir) EnsureDirectory(elem string, elems ...string) (*Dir, error) {
	if !isValidSinglePathComponent(elem) {
		return nil, errors.Reason("invalid path component #0: %(comp)q").
			D("comp", elem).Err()
	}

	// No element may have a path separator in it.
	components := make([]string, 0, len(elems)+2)
	components = append(components, d.String())
	components = append(components, elem)
	for i, comp := range elems {
		if !isValidSinglePathComponent(comp) {
			return nil, errors.Reason("invalid path component #%(index)d: %(comp)q").
				D("index", i+1).D("comp", comp).Err()
		}
		components = append(components, comp)
	}

	// Make the full subdirectory path.
	fullPath := filepath.Join(components...)
	if err := ensureDirectory(fullPath); err != nil {
		return nil, errors.Annotate(err).Reason("failed to create directory [%(path)s]").D("path", fullPath).Err()
	}

	// Return the last managed directory node.
	return d.GetSubDirectory(elem, elems...), nil
}

// ShallowSymlinkFrom creates one symlink under d per regular file in dir.
func (d *Dir) ShallowSymlinkFrom(dir string, rel bool) error {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Annotate(err).Reason("failed to read directory [%(path)s]").D("path", dir).Err()
	}

	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}

		f := d.File(fi.Name())
		if err := f.SymlinkFrom(filepath.Join(dir, fi.Name()), rel); err != nil {
			return err
		}
	}

	return nil
}

// File creates a new managed File immediately within d.
func (d *Dir) File(name string) *File {
	if !isValidSinglePathComponent(name) {
		panic(errors.Reason("invalid path component: %(name)q").D("name", name).Err())
	}

	// Register this file with its managed directory.
	return d.registerFile(name)
}

func (d *Dir) registerFile(name string) *File {
	d.fs.lock.RLock()
	_, registered := d.files[name]
	d.fs.lock.RUnlock()

	if !registered {
		d.fs.lock.Lock()
		defer d.fs.lock.Unlock()

		if d.files == nil {
			d.files = make(map[string]struct{})
		}
		d.files[name] = struct{}{}
	}

	return &File{
		dir:  d,
		name: name,
	}
}

// CleanUp deletes any unmanaged files within this directory and its
// descendants.
//
// The filesystem's lock is held for the duration of the cleanup.
func (d *Dir) CleanUp() error {
	d.fs.lock.Lock()
	defer d.fs.lock.Unlock()

	// If any of our parents is ignored, do no cleanup.
	for p := d; p != nil; p = p.parent {
		if p.ignored {
			return nil
		}
	}

	return d.cleanupLocked()
}

func (d *Dir) cleanupLocked() error {
	if d.ignored {
		return nil
	}

	// List the contents of this directory.
	fis, err := ioutil.ReadDir(d.String())
	if err != nil {
		return errors.Annotate(err).Reason("failed to read directory %(dir)q").D("dir", d.String()).Err()
	}

	for _, fi := range fis {
		name := fi.Name()
		if fi.IsDir() {
			if subD := d.subdirs[name]; subD != nil {
				// Cleanup our subdirectory.
				if err := subD.cleanupLocked(); err != nil {
					return err
				}
				continue
			}
		} else {
			// This is a file/symlink/etc. Delete it if it's not managed.
			if _, ok := d.files[name]; ok {
				continue
			}
		}

		// This is unmanaged. Delete it.
		fiPath := filepath.Join(d.String(), name)

		// Safety / sanity check, the path under consideration for deletion MUST be
		// underneath of our filesystem's root.
		if !isSubpath(d.fs.rootDir, fiPath) {
			panic(errors.Reason("about to delete [%(path)q], which is not a subdirectory of [%(root)q]").
				D("path", fiPath).D("root", d.fs.rootDir).Err())
		}

		if err := os.RemoveAll(fiPath); err != nil {
			return errors.Annotate(err).Reason("failed to delete unmanaged [%(path)q]").D("path", fiPath).Err()
		}
	}
	return nil
}

// File is a managed file within a managed Dir.
type File struct {
	dir *Dir

	name string
}

// String returns the full path of this File.
func (f *File) String() string { return filepath.Join(f.dir.String(), f.name) }

// RelPath returns the path of this File relative to its filesystem root.
func (f *File) RelPath() string { return filepath.Join(f.dir.relPath, f.name) }

// Name returns the last path component to this file, it's file name.
func (f *File) Name() string {
	return f.name
}

// SymlinkFrom writes a symlink from the specified path to f's location.
//
// If rel is true, the symlink's target will be "from" relative to the managed
// filesystem's root. If it goes above the root, it is an error.
func (f *File) SymlinkFrom(from string, rel bool) error {
	to := f.String()

	// Assert that our "from" field exists.
	if _, err := os.Stat(from); err != nil {
		return errors.Annotate(err).Reason("failed to stat symlink from [%(path)s]").
			D("path", from).Err()
	}

	// "from" must be a path under the filesystem base.
	if rel {
		var err error
		if from, err = f.dir.RelPathFrom(from); err != nil {
			return errors.Annotate(err).Reason("failed to calculate relative path").Err()
		}
	}

	if st, err := os.Lstat(to); err == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			// to is a symlink. Is it a symlink to "from"?
			currentFrom, err := os.Readlink(to)
			if err == nil && currentFrom == from {
				// The symlink already exists!
				return nil
			}
		}

		// The file exists, but is not a symlink to "from". Try to delete it.
		if err := os.Remove(to); err != nil {
			return err
		}
	}

	if err := os.Symlink(from, to); err != nil {
		return errors.Annotate(err).Reason("failed to symlink [%(from)s] => [%(to)s]").
			D("from", from).D("to", to).Err()
	}
	return nil
}

// CopyFrom copies contents from the specified path to f's location.
func (f *File) CopyFrom(src string) (rerr error) {
	dst := f.String()

	s, err := os.Open(src)
	if err != nil {
		return errors.Annotate(err).Reason("failed to open [%(source)s]").
			D("source", src).Err()
	}
	defer func() {
		cerr := s.Close()
		if rerr == nil && cerr != nil {
			rerr = cerr
		}
	}()

	d, err := createFile(dst)
	if err != nil {
		return errors.Annotate(err).Reason("failed to create [%(dest)s]").
			D("dest", dst).Err()
	}
	defer func() {
		cerr := d.Close()
		if rerr == nil && cerr != nil {
			rerr = cerr
		}
	}()

	withBuffer(func(buf []byte) {
		_, err = io.CopyBuffer(d, s, buf)
	})
	if err != nil {
		return errors.Annotate(err).Reason("failed to copy from [%(source)s] => [%(dest)s]").
			D("source", src).D("dest", dst).Err()
	}
	return nil
}

// SameAs returns true if the spceified path is the exact same file as the
// file at f's location.
func (f *File) SameAs(other string) (bool, error) {
	// Stat each file and check if they are the same.
	myPath := f.String()
	myStat, err := os.Lstat(myPath)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, errors.Annotate(err).Reason("failed to stat [%(self)s]").
			D("self", myPath).Err()
	}

	otherStat, err := os.Lstat(other)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, errors.Annotate(err).Reason("failed to stat [%(other)s]").
			D("other", other).Err()
	}

	if os.SameFile(myStat, otherStat) {
		return true, nil
	}

	// Not the same file, and both exist, so let's compare byte-for-byte.
	myFD, err := os.Open(f.String())
	if err != nil {
		return false, errors.Annotate(err).Reason("failed to open [%(self)s]").
			D("self", myPath).Err()
	}
	defer myFD.Close()

	otherFD, err := os.Open(other)
	if err != nil {
		return false, errors.Annotate(err).Reason("failed to option [%(other)s]").
			D("other", other).Err()
	}
	defer otherFD.Close()

	same := false
	withBuffer(func(myBuf []byte) {
		withBuffer(func(otherBuf []byte) {
			same, err = byteCompare(myFD, otherFD, myBuf, otherBuf)
		})
	})
	if err != nil {
		return false, errors.Annotate(err).Reason("failed to compare files [%(self)s] to [$(other)s]").
			D("self", myPath).D("other", other).Err()
	}
	return same, nil
}

// GenerateTextProto writes the specified object as rendered text protobuf to
// f's location.
func (f *File) GenerateTextProto(c context.Context, msg proto.Message) error {
	return f.WriteDataFrom(func(fd io.Writer) error {
		if gh := f.dir.fs.GenHeader; len(gh) > 0 {
			for _, h := range gh {
				if _, err := fmt.Fprint(fd, "# "); err != nil {
					return err
				}

				if _, err := fmt.Fprint(fd, h); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(fd, "\n"); err != nil {
				return err
			}
		}

		if gn := f.dir.fs.GenName; gn != "" {
			genNote := fmt.Sprintf("# GENERATED by %q at: %s\n\n", gn, clock.Now(c))
			if _, err := fmt.Fprint(fd, genNote); err != nil {
				return err
			}
		}

		// Write YAML contents.
		if err := proto.MarshalText(fd, msg); err != nil {
			return errors.Annotate(err).Err()
		}
		return nil
	})
}

// GenerateYAML writes the specified object as a YAML file to f's location.
func (f *File) GenerateYAML(c context.Context, obj interface{}) error {
	d, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}

	return f.WriteDataFrom(func(fd io.Writer) error {
		if gh := f.dir.fs.GenHeader; len(gh) > 0 {
			for _, h := range gh {
				if _, err := fmt.Fprint(fd, "# "); err != nil {
					return err
				}

				if _, err := fmt.Fprint(fd, h); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(fd, "\n"); err != nil {
				return err
			}
		}

		if gn := f.dir.fs.GenName; gn != "" {
			genNote := fmt.Sprintf("# GENERATED by %q at: %s\n\n", gn, clock.Now(c))
			if _, err := fmt.Fprint(fd, genNote); err != nil {
				return err
			}
		}

		// Write YAML contents.
		if _, err := fd.Write(d); err != nil {
			return errors.Annotate(err).Err()
		}
		return nil
	})
}

// GenerateGo writes Go contents to the supplied filesystem.
func (f *File) GenerateGo(c context.Context, content string) error {
	return f.WriteDataFrom(func(fd io.Writer) error {
		if gh := f.dir.fs.GenHeader; len(gh) > 0 {
			for _, h := range gh {
				if _, err := fmt.Fprint(fd, "// "); err != nil {
					return err
				}

				if _, err := fmt.Fprint(fd, h); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(fd, "\n"); err != nil {
				return err
			}
		}

		if gn := f.dir.fs.GenName; gn != "" {
			genNote := fmt.Sprintf("// GENERATED by %q at: %s\n\n", gn, clock.Now(c))
			if _, err := fmt.Fprint(fd, genNote); err != nil {
				return err
			}
		}

		// Write Go content.
		if _, err := fmt.Fprint(fd, content); err != nil {
			return errors.Annotate(err).Err()
		}
		return nil
	})
}

// ReadAll reads the contents of the file at f's location.
func (f *File) ReadAll() ([]byte, error) {
	return ioutil.ReadFile(f.String())
}

// WriteData writes the supplied bytes to f's location.
func (f *File) WriteData(d []byte) (err error) {
	return f.WriteDataFrom(func(w io.Writer) error {
		_, err := w.Write(d)
		return err
	})
}

// WriteDataFrom creates a new file at f's location and passes a Writer to this
// file to the supplied callback.
func (f *File) WriteDataFrom(fn func(io.Writer) error) (err error) {
	var fd *os.File
	if fd, err = createFile(f.String()); err != nil {
		return
	}
	defer func() {
		if closeErr := fd.Close(); err == nil {
			err = closeErr
		}
	}()

	err = fn(fd)
	return
}
