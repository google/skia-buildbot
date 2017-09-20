package util

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func forZipFile(r *zip.ReadCloser, fn func(f *zip.File, rc io.ReadCloser) error) error {
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		if err := fn(f, rc); err != nil {
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

// UnZip unzips the file specified in src into the 'dest' directory.
// Note: The parent directory will be created using the mode of the first directory in the zip.
func UnZip(dest, src string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}

		fn := func(f *zip.File, rc io.ReadCloser) error {
			path := filepath.Join(dest, f.Name)
			if f.FileInfo().IsDir() {
				if err := os.MkdirAll(path, f.Mode()); err != nil {
					return err
				}
			} else {
				f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
				if err != nil {
					return err
				}
				if _, err = io.Copy(f, rc); err != nil {
					return err
				}
				if err := f.Close(); err != nil {
					return err
				}
			}
			return nil
		}

		if err := forZipFile(r, fn); err != nil {
			return err
		}
		return rc.Close()
	}

	return r.Close()
}

// ZipIt zips the specified directory into the target file.
// Note: source must be an absolute path. Also, this is untested with symlinks.
func ZipIt(target, source string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}

	archive := zip.NewWriter(zipfile)

	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name, err = filepath.Rel(source, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err = io.Copy(writer, file); err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("Failed to close file %s: %s", file.Name(), err)
		}
		return nil
	})

	if err := archive.Close(); err != nil {
		return fmt.Errorf("Failed to close writer to zipfile %s: %s", zipfile.Name(), err)
	}
	if err := zipfile.Close(); err != nil {
		return fmt.Errorf("Failed to close the zipfile %s: %s", zipfile.Name(), err)
	}

	return err
}
