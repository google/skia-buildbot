package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/blakesmith/ar"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

func main() {
	var (
		input     = flag.String("input", "", "[required] The .ar or .deb file to extract")
		outputDir = flag.String("output_dir", ".", "The path to write the files to")
	)
	flag.Parse()

	if *input == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	err := extract(*input, *outputDir)
	if err != nil {
		fmt.Printf("Could not extract contents from ar archive %s : %s\n", *input, err)
	}
}
func extract(input, outputDir string) error {
	return util.WithReadFile(input, func(r io.Reader) error {
		archive := ar.NewReader(r)
		var err error
		for hdr, err := archive.Next(); err == nil; hdr, err = archive.Next() {
			output := filepath.Join(outputDir, hdr.Name)
			err := util.WithWriteFile(output, func(w io.Writer) error {
				// This will read all the bytes in the current file that the archive is looking at
				// to the output file.
				_, err := io.Copy(w, archive)
				return skerr.Wrap(err)
			})
			if err != nil {
				return skerr.Wrapf(err, "creating output file %s", output)
			}
		}
		if err != io.EOF {
			return skerr.Wrap(err)
		}
		return nil
	})
}
