package zip

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func forZipFile(zipRC *zip.ReadCloser, fn func(f *zip.File, r io.Reader) error) error {
	for _, f := range zipRC.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		if err := fn(f, rc); err != nil {
			_ = rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

// UnZip unzips the file specified in src into the 'dest' directory.
// Note:
// * The parent directory will be created using the mode of the first directory in the zip.
// * src could be partially populated if an error is returned.
func UnZip(dest, src string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}

	fn := func(f *zip.File, r io.Reader) error {
		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
		} else {
			contents, err := ioutil.ReadAll(r)
			if err != nil {
				return err
			}
			return ioutil.WriteFile(path, contents, f.Mode())
		}
		return nil
	}

	if err := forZipFile(r, fn); err != nil {
		_ = r.Close()
		return err
	}

	return r.Close()
}

// Directory zips the specified directory into the target file.
// Note: source must be an absolute path. Also, this is untested with symlinks.
func Directory(target, source string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}

	archive := zip.NewWriter(zipfile)

	walkErr := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
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
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("Failed to close file %s: %s", file.Name(), err)
		}
		return nil
	})

	if err := archive.Close(); err != nil {
		_ = zipfile.Close()
		return fmt.Errorf("Failed to close writer to zipfile %s: %s : %v", zipfile.Name(), err, walkErr)
	}
	if err := zipfile.Close(); err != nil {
		return fmt.Errorf("Failed to close the zipfile %s: %s : %v", zipfile.Name(), err, walkErr)
	}

	return walkErr
}
