package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	generatedFileHeaderTmpl = `// This file was generated from %s.  DO NOT EDIT.

`
)

var (
	protoMarshalOptions = prototext.MarshalOptions{
		Multiline: true,
	}
	funcMap = template.FuncMap{
		"map": makeMap,
	}
)

func main() {
	src := flag.String("src", "", "Source directory.")
	dst := flag.String("dst", "", "Destination directory. Outputs will mimic the structure of the source.")

	flag.Parse()

	if *src == "" {
		sklog.Fatal("--src is required.")
	}
	if *dst == "" {
		sklog.Fatal("--dst is required.")
	}

	// Retrieve the config vars.
	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	reg, err := config_vars.NewRegistry(ctx, chrome_branch.NewClient(client))
	if err != nil {
		sklog.Fatal(err)
	}

	// Find templates in the given source directory.
	fsys := os.DirFS(*src)
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tmpl") {
			if err := process(ctx, path, *src, *dst, reg.Vars()); err != nil {
				return skerr.Wrapf(err, "failed to convert config %s", path)
			}
		}
		return nil
	}); err != nil {
		sklog.Fatalf("Failed to read configs: %s", err)
	}
}

func process(ctx context.Context, relPath, srcDir, dstDir string, vars *config_vars.Vars) error {
	// Read and execute the template.
	srcPath := filepath.Join(srcDir, relPath)
	sklog.Infof("Reading %s from %s", relPath, srcDir)
	tmpl, err := template.New(filepath.Base(srcPath)).Funcs(funcMap).ParseFiles(srcPath)
	if err != nil {
		return skerr.Wrapf(err, "failed to parse template file %q", srcPath)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return skerr.Wrapf(err, "failed to execute template file %q", srcPath)
	}

	// Parse the template as a list of Configs.
	var configs config.Configs
	if err := prototext.Unmarshal(buf.Bytes(), &configs); err != nil {
		return skerr.Wrapf(err, "failed to parse config proto from template file %q", srcPath)
	}
	dstBase, _ := filepath.Split(relPath)
	dstDir = filepath.Join(dstDir, dstBase)
	sklog.Infof("  Found %d configs in %s", len(configs.Config), srcPath)
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}
	headerBytes := []byte(fmt.Sprintf(generatedFileHeaderTmpl, relPath))
	for _, cfg := range configs.Config {
		encBytes, err := protoMarshalOptions.Marshal(cfg)
		if err != nil {
			return skerr.Wrapf(err, "failed to encode config from %q", srcPath)
		}
		dstPath := filepath.Join(dstDir, cfg.RollerName+".cfg")
		if err := ioutil.WriteFile(dstPath, append(headerBytes, encBytes...), 0644); err != nil {
			return skerr.Wrapf(err, "failed to write config file %q", dstPath)
		}
		sklog.Infof("  Wrote %s", dstPath)
	}
	return nil
}

func makeMap(elems ...interface{}) (map[string]interface{}, error) {
	if len(elems)%2 != 0 {
		return nil, skerr.Fmt("Requires an even number of elements, not %d", len(elems))
	}
	rv := make(map[string]interface{}, len(elems)/2)
	for i := 0; i < len(elems); i += 2 {
		key, ok := elems[i].(string)
		if !ok {
			return nil, skerr.Fmt("Map keys must be strings, not %v", elems[i])
		}
		rv[key] = elems[i+1]
	}
	return rv, nil
}
