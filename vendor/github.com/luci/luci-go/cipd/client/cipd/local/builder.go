// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/cipd/client/cipd/common"
	"github.com/luci/luci-go/common/logging"
)

// BuildInstanceOptions defines options for BuildInstance function.
type BuildInstanceOptions struct {
	// Input is a list of files to add to the package.
	Input []File

	// Output is where to write the package file to.
	Output io.Writer

	// PackageName is name of the package being built, e.g. 'infra/tools/cipd'.
	PackageName string

	// VersionFile is slash separated path where to drop JSON with version info.
	VersionFile string

	// InstallMode defines how to install the package: "copy" or "symlink".
	InstallMode InstallMode
}

// BuildInstance builds a new package instance.
//
// If build an instance of package named opts.PackageName by archiving input
// files (passed via opts.Input).
//
// The final binary is written to opts.Output. Some output may be written even
// if BuildInstance eventually returns an error.
func BuildInstance(ctx context.Context, opts BuildInstanceOptions) error {
	err := common.ValidatePackageName(opts.PackageName)
	if err != nil {
		return err
	}

	// Make sure no files are written to package service directory.
	for _, f := range opts.Input {
		if strings.HasPrefix(f.Name(), packageServiceDir+"/") {
			return fmt.Errorf("can't write to %s: %s", packageServiceDir, f.Name())
		}
	}

	// Generate the manifest file, add to the list of input files.
	manifestFile, err := makeManifestFile(opts)
	if err != nil {
		return err
	}
	files := append(opts.Input, manifestFile)

	// Make sure filenames are unique.
	seenNames := make(map[string]struct{}, len(files))
	for _, f := range files {
		_, seen := seenNames[f.Name()]
		if seen {
			return fmt.Errorf("file %s is provided twice", f.Name())
		}
		seenNames[f.Name()] = struct{}{}
	}

	// Write the final zip file.
	return zipInputFiles(ctx, files, opts.Output)
}

// zipInputFiles deterministically builds a zip archive out of input files and
// writes it to the writer. Files are written in the order given.
func zipInputFiles(ctx context.Context, files []File, w io.Writer) error {
	writer := zip.NewWriter(w)
	defer writer.Close()

	// Reports zipping progress to the log each second.
	lastReport := time.Time{}
	progress := func(count int) {
		if time.Since(lastReport) > time.Second {
			lastReport = time.Now()
			logging.Infof(ctx, "Zipping files: %d files left", len(files)-count)
		}
	}

	for i, in := range files {
		progress(i)

		// Bail out early if context is canceled.
		if err := ctx.Err(); err != nil {
			return err
		}

		// Intentionally do not add timestamp or file mode to make zip archive
		// deterministic. See also zip.FileInfoHeader() implementation.
		fh := zip.FileHeader{
			Name:   in.Name(),
			Method: zip.Deflate,
		}

		mode := os.FileMode(0600)
		if in.Executable() {
			mode |= 0100
		}
		if in.Symlink() {
			mode |= os.ModeSymlink
		}
		fh.SetMode(mode)

		dst, err := writer.CreateHeader(&fh)
		if err != nil {
			return err
		}
		if in.Symlink() {
			err = zipSymlinkFile(dst, in)
		} else {
			err = zipRegularFile(dst, in)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func zipRegularFile(dst io.Writer, f File) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	written, err := io.Copy(dst, src)
	if err != nil {
		return err
	}
	if uint64(written) != f.Size() {
		return fmt.Errorf("file %s changed midway", f.Name())
	}
	return nil
}

func zipSymlinkFile(dst io.Writer, f File) error {
	target, err := f.SymlinkTarget()
	if err != nil {
		return err
	}
	// Symlinks are zipped as text files with target path. os.ModeSymlink bit in
	// the header distinguishes them from regular files.
	_, err = dst.Write([]byte(target))
	return err
}

////////////////////////////////////////////////////////////////////////////////

type manifestFile []byte

func (m *manifestFile) Name() string     { return manifestName }
func (m *manifestFile) Size() uint64     { return uint64(len(*m)) }
func (m *manifestFile) Executable() bool { return false }
func (m *manifestFile) Symlink() bool    { return false }

func (m *manifestFile) SymlinkTarget() (string, error) {
	return "", fmt.Errorf("not a symlink: %s", m.Name())
}

func (m *manifestFile) Open() (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader(*m)), nil
}

// makeManifestFile generates a package manifest file and returns it as
// File interface.
func makeManifestFile(opts BuildInstanceOptions) (File, error) {
	if opts.VersionFile != "" && !isCleanSlashPath(opts.VersionFile) {
		return nil, fmt.Errorf("version file path should be a clean path relative to a package root: %s", opts.VersionFile)
	}
	if err := ValidateInstallMode(opts.InstallMode); err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	err := writeManifest(&Manifest{
		FormatVersion: manifestFormatVersion,
		PackageName:   opts.PackageName,
		VersionFile:   opts.VersionFile,
		InstallMode:   opts.InstallMode,
	}, buf)
	if err != nil {
		return nil, err
	}
	out := manifestFile(buf.Bytes())
	return &out, nil
}
