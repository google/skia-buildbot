package tar

/*
   Utility for working with tar archives.
*/

import (
	"archive/tar"
	"compress/gzip"
	"io"
)

// ReadArchive reads a tar archive from the given io.Reader and runs the given
// callback function for each file in the archive.
func ReadArchive(archive io.Reader, cb func(string, io.Reader) error) error {
	r := tar.NewReader(archive)
	for {
		header, err := r.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeReg {
			if err := cb(header.Name, r); err != nil {
				return err
			}
		}
	}
}

// ReadGzipArchive reads a gzipped tar archive from the given io.Reader and runs
// the given callback function for each file in the archive.
func ReadGzipArchive(archive io.Reader, cb func(string, io.Reader) error) error {
	r, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	return ReadArchive(r, cb)
}
