// Use: go build create.go && ./create --asset_name clang_linux -root /tmp/assets/clang_linux

package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"go.skia.org/infra/go/sklog"
)

var (
	assetName = flag.String("asset_name", "", "Name used to refer to the asset in Bazel rule, will produce <asset_name>_manifest array in <asset_name>.bzl")
	root      = flag.String("root", "", "Root directory of assets to catalog.")
	dryrun    = flag.Bool("dryrun", false, "Print manifest output to stdout rather than file.")
)

func main() {
	flag.Parse()
	if *assetName == "" {
		sklog.Fatal("asset_name required")
	}
	if *root == "" {
		sklog.Fatal("root required")
	}
	// Clean so that /path/to/dir and /path/to/dir/ are canonicalized to path/to/dir.
	aboveRoot := path.Dir(path.Clean(*root))
	var filenames []string
	filepath.Walk(*root, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			sklog.Fatal(err)
			return err
		}
		if info.IsDir() {
			// In case this asset was download from CIPD, don't catalog those files.
			if info.Name() == ".cipd" {
				return filepath.SkipDir
			}
			return nil
		}
		// TODO(westont): check for symbolic links and fail.
		if !info.IsDir() {
			path, err := filepath.Rel(aboveRoot, file)
			if err != nil {
				sklog.Fatal(err)
				return err
			}
			filenames = append(filenames, path)
		}
		return nil
	})

	template, err := template.New("bzlFil").Parse(`{{.Docstring}}

{{.Variable}} = [
{{range .Files}}	"{{.}}",
{{end}}	]
`)

	if err != nil {
		sklog.Fatal(err)
	}

	type fileList struct {
		Docstring string
		Variable  string
		Files     []string
	}
	data := fileList{
		Docstring: fmt.Sprintf("\"Manifest for %s, to be used as output for a cipd_package rule.\"", *assetName),
		Variable:  fmt.Sprintf("%s_manifest", *assetName),
		Files:     filenames,
	}
	if *dryrun {
		err = template.Execute(os.Stdout, data)
	} else {
		f, err := os.Create(*assetName + "_manifest.bzl")
		if err != nil {
			sklog.Fatal(err)
		}
		defer f.Close()
		err = template.Execute(f, data)
	}
	if err != nil {
		sklog.Fatal(err)
	}

	fmt.Printf("Number of files added to manifest: %d\n", len(filenames))
}
