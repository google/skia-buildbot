// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/cipd/client/cipd/common"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
)

// PackageInstance represents a binary CIPD package file (with manifest inside).
type PackageInstance interface {
	// Close shuts down the package and its data provider.
	Close() error

	// Pin identifies package name and concreted instance ID of this package file.
	Pin() common.Pin

	// Files returns a list of files to deploy with the package.
	Files() []File

	// DataReader returns reader that reads raw package data.
	DataReader() io.ReadSeeker
}

// OpenInstance verifies the package and prepares it for extraction.
//
// It checks package SHA1 hash (must match instanceID, if it's given) and
// prepares a package instance for extraction. If the call succeeds,
// PackageInstance takes ownership of io.ReadSeeker. If it also implements
// io.Closer, it will be closed when package.Close() is called. If an error is
// returned, io.ReadSeeker remains unowned and caller is responsible for closing
// it (if required).
func OpenInstance(ctx context.Context, r io.ReadSeeker, instanceID string) (PackageInstance, error) {
	out := &packageInstance{data: r}
	if err := out.open(instanceID); err != nil {
		return nil, err
	}
	return out, nil
}

// OpenInstanceFile opens a package instance file on disk.
func OpenInstanceFile(ctx context.Context, path string, instanceID string) (inst PackageInstance, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	inst, err = OpenInstance(ctx, file, instanceID)
	if err != nil {
		file.Close()
	}
	return
}

// ExtractInstance extracts all files from a package instance into a destination.
func ExtractInstance(ctx context.Context, inst PackageInstance, dest Destination) error {
	if err := dest.Begin(ctx); err != nil {
		return err
	}

	// Do not leave garbage around in case of a panic.
	needToEnd := true
	defer func() {
		if needToEnd {
			dest.End(ctx, false)
		}
	}()

	files := inst.Files()

	extractManifestFile := func(f File) (err error) {
		manifest, err := readManifestFile(f)
		if err != nil {
			return err
		}
		manifest.Files = make([]FileInfo, 0, len(files))
		for _, file := range files {
			// Do not put info about service .cipdpkg files into the manifest,
			// otherwise it becomes recursive and "size" property of manifest file
			// itself is not correct.
			if strings.HasPrefix(file.Name(), packageServiceDir+"/") {
				continue
			}
			fi := FileInfo{
				Name:       file.Name(),
				Size:       file.Size(),
				Executable: file.Executable(),
			}
			if file.Symlink() {
				target, err := file.SymlinkTarget()
				if err != nil {
					return err
				}
				fi.Symlink = target
			}
			manifest.Files = append(manifest.Files, fi)
		}
		out, err := dest.CreateFile(ctx, f.Name(), false)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := out.Close(); err == nil {
				err = closeErr
			}
		}()
		return writeManifest(&manifest, out)
	}

	extractSymlinkFile := func(f File) error {
		target, err := f.SymlinkTarget()
		if err != nil {
			return err
		}
		return dest.CreateSymlink(ctx, f.Name(), target)
	}

	extractRegularFile := func(f File) (err error) {
		out, err := dest.CreateFile(ctx, f.Name(), f.Executable())
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := out.Close(); err == nil {
				err = closeErr
			}
		}()
		in, err := f.Open()
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(out, in)
		return err
	}

	// Use file sizes for progress report calculation.
	totalSize := uint64(0)
	extractedSize := uint64(0)
	for _, f := range files {
		if !f.Symlink() {
			totalSize += f.Size()
		}
	}

	logging.Infof(ctx, "cipd: about to extract %.1f Mb", float64(totalSize)/1024.0/1024.0)

	// reportProgress print extraction progress, throttling the reports rate to
	// one per 5 sec.
	prevProgress := 1000 // >100%
	var prevReportTs time.Time
	reportProgress := func(read, total uint64) {
		now := clock.Now(ctx)
		progress := int(float64(read) * 100 / float64(total))
		if progress < prevProgress || read == total || now.Sub(prevReportTs) > 5*time.Second {
			logging.Infof(ctx, "cipd: extracting - %d%%", progress)
			prevReportTs = now
			prevProgress = progress
		}
	}

	var err error
	for _, f := range files {
		if err = ctx.Err(); err != nil {
			break
		}
		if f.Name() == manifestName {
			err = extractManifestFile(f)
			extractedSize += f.Size()
		} else if f.Symlink() {
			err = extractSymlinkFile(f)
		} else {
			err = extractRegularFile(f)
			extractedSize += f.Size()
		}
		if err != nil {
			break
		}
		reportProgress(extractedSize, totalSize)
	}

	needToEnd = false
	if err == nil {
		err = dest.End(ctx, true)
	} else {
		// Ignore error in 'End' and return the original error.
		dest.End(ctx, false)
	}

	return err
}

////////////////////////////////////////////////////////////////////////////////
// PackageInstance implementation.

type packageInstance struct {
	data       io.ReadSeeker
	dataSize   int64
	instanceID string
	zip        *zip.Reader
	files      []File
	manifest   Manifest
}

// open reads the package data, verifies SHA1 hash and reads manifest.
func (inst *packageInstance) open(instanceID string) error {
	// Calculate SHA1 of the data to verify it matches expected instanceID.
	if _, err := inst.data.Seek(0, os.SEEK_SET); err != nil {
		return err
	}
	hash := sha1.New()
	if _, err := io.Copy(hash, inst.data); err != nil {
		return err
	}

	dataSize, err := inst.data.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	inst.dataSize = dataSize

	calculatedSHA1 := hex.EncodeToString(hash.Sum(nil))
	if instanceID != "" && instanceID != calculatedSHA1 {
		return fmt.Errorf("package SHA1 hash mismatch")
	}
	inst.instanceID = calculatedSHA1

	// List files and package manifest.
	inst.zip, err = zip.NewReader(&readerAt{r: inst.data}, inst.dataSize)
	if err != nil {
		return err
	}
	inst.files = make([]File, len(inst.zip.File))
	for i, zf := range inst.zip.File {
		inst.files[i] = &fileInZip{z: zf}
		if inst.files[i].Name() == manifestName {
			inst.manifest, err = readManifestFile(inst.files[i])
			if err != nil {
				return err
			}
		}
	}

	// Generate version_file if needed.
	if inst.manifest.VersionFile != "" {
		vf, err := makeVersionFile(inst.manifest.VersionFile, VersionFile{
			PackageName: inst.manifest.PackageName,
			InstanceID:  inst.instanceID,
		})
		if err != nil {
			return err
		}
		inst.files = append(inst.files, vf)
	}

	return nil
}

func (inst *packageInstance) Close() error {
	if inst.data != nil {
		if closer, ok := inst.data.(io.Closer); ok {
			closer.Close()
		}
		inst.data = nil
	}
	inst.dataSize = 0
	inst.instanceID = ""
	inst.zip = nil
	inst.files = []File{}
	inst.manifest = Manifest{}
	return nil
}

func (inst *packageInstance) Pin() common.Pin {
	return common.Pin{
		PackageName: inst.manifest.PackageName,
		InstanceID:  inst.instanceID,
	}
}

func (inst *packageInstance) Files() []File             { return inst.files }
func (inst *packageInstance) DataReader() io.ReadSeeker { return inst.data }

////////////////////////////////////////////////////////////////////////////////
// Utilities.

// readManifestFile decodes manifest file zipped inside the package.
func readManifestFile(f File) (Manifest, error) {
	r, err := f.Open()
	if err != nil {
		return Manifest{}, err
	}
	defer r.Close()
	return readManifest(r)
}

// makeVersionFile returns File representing a JSON blob with info about package
// version. It's what's deployed at path specified in 'version_file' stanza in
// package definition YAML.
func makeVersionFile(relPath string, versionFile VersionFile) (File, error) {
	if !isCleanSlashPath(relPath) {
		return nil, fmt.Errorf("invalid version_file: %s", relPath)
	}
	blob, err := json.MarshalIndent(versionFile, "", "  ")
	if err != nil {
		return nil, err
	}
	return &blobFile{
		name: relPath,
		blob: blob,
	}, nil
}

// blobFile implements File on top of byte array with file data.
type blobFile struct {
	name string
	blob []byte
}

func (b *blobFile) Name() string                   { return b.name }
func (b *blobFile) Size() uint64                   { return uint64(len(b.blob)) }
func (b *blobFile) Executable() bool               { return false }
func (b *blobFile) Symlink() bool                  { return false }
func (b *blobFile) SymlinkTarget() (string, error) { return "", nil }

func (b *blobFile) Open() (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader(b.blob)), nil
}

////////////////////////////////////////////////////////////////////////////////
// File interface implementation via zip.File.

type fileInZip struct {
	z *zip.File
}

func (f *fileInZip) Name() string  { return f.z.Name }
func (f *fileInZip) Symlink() bool { return (f.z.Mode() & os.ModeSymlink) != 0 }

func (f *fileInZip) Executable() bool {
	if f.Symlink() {
		return false
	}
	return (f.z.Mode() & 0100) != 0
}

func (f *fileInZip) Size() uint64 {
	if f.Symlink() {
		return 0
	}
	return f.z.UncompressedSize64
}

func (f *fileInZip) SymlinkTarget() (string, error) {
	if !f.Symlink() {
		return "", fmt.Errorf("not a symlink: %s", f.Name())
	}
	r, err := f.z.Open()
	if err != nil {
		return "", err
	}
	defer r.Close()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *fileInZip) Open() (io.ReadCloser, error) {
	if f.Symlink() {
		return nil, fmt.Errorf("opening a symlink is not allowed: %s", f.Name())
	}
	return f.z.Open()
}

////////////////////////////////////////////////////////////////////////////////
// ReaderAt implementation via ReadSeeker. Not concurrency safe, moves file
// pointer around without any locking. Works OK in the context of OpenInstance
// function though (where OpenInstance takes sole ownership of io.ReadSeeker).

type readerAt struct {
	r io.ReadSeeker
}

func (r *readerAt) ReadAt(data []byte, off int64) (int, error) {
	_, err := r.r.Seek(off, os.SEEK_SET)
	if err != nil {
		return 0, err
	}
	return r.r.Read(data)
}
