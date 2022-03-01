package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/kube_conf_gen_lib"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	// Parent repo name for Google3 rollers.
	GOOGLE3_PARENT_NAME = "Google3"
)

var (
	// backendTemplate is the template used to generate the k8s YAML config file
	// for autoroll backends.
	//go:embed autoroll-be.yaml.template
	backendTemplate string
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
	if backendTemplate == "" {
		sklog.Fatal("internal error; embedded template is empty.")
	}

	ctx := context.Background()
	fsys := os.DirFS(*src)
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".cfg") {
			if err := convertConfig(ctx, path, *src, *dst); err != nil {
				return skerr.Wrapf(err, "failed to convert config %s", path)
			}
		}
		return nil
	}); err != nil {
		sklog.Fatalf("Failed to read configs: %s", err)
	}
}

func convertConfig(ctx context.Context, relPath, srcDir, dstDir string) error {
	// Read the config file.
	srcPath := filepath.Join(srcDir, relPath)
	cfgBytes, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return skerr.Wrapf(err, "failed to read roller config %s", srcPath)
	}
	var cfg config.Config
	if err := prototext.Unmarshal(cfgBytes, &cfg); err != nil {
		sklog.Fatalf("failed to parse roller config %s: %s", srcPath, err)
	}
	// Google3 uses a different type of backend.
	if cfg.ParentDisplayName == GOOGLE3_PARENT_NAME {
		return nil
	}

	// kube-conf-gen wants a JSON-ish version of the config in order to build
	// the config map.
	cfgJsonBytes, err := protojson.MarshalOptions{
		AllowPartial:    true,
		EmitUnpopulated: true,
	}.Marshal(&cfg)
	if err != nil {
		return skerr.Wrap(err)
	}
	cfgJson := map[string]interface{}{}
	if err := json.Unmarshal(cfgJsonBytes, &cfgJson); err != nil {
		return skerr.Wrap(err)
	}
	cfgMap := map[string]interface{}{}
	if err := kube_conf_gen_lib.ParseConfigHelper(cfgJson, cfgMap, false); err != nil {
		return skerr.Wrap(err)
	}

	// Encode the roller config file as base64.
	// Note that we could re-read the config file from disk
	// and base64-encode its contents. In practice, the
	// behavior of the autoroll frontend and backends would
	// be the same, so we consider it preferable to encode
	// the parsed config, which will strip things like
	// comments and whitespace that would otherwise produce
	// a "different" config.
	b, err := prototext.MarshalOptions{
		AllowPartial: true,
		EmitUnknown:  true,
	}.Marshal(&cfg)
	if err != nil {
		return skerr.Wrapf(err, "Failed to encode roller config as text proto")
	}
	cfgFileBase64 := base64.StdEncoding.EncodeToString(b)
	cfgMap["configBase64"] = cfgFileBase64

	// Run kube-conf-gen to generate the output file.
	relDir, baseName := filepath.Split(relPath)
	dstPath := filepath.Join(dstDir, relDir, fmt.Sprintf("autoroll-be-%s.yaml", strings.Split(baseName, ".")[0]))
	if err := kube_conf_gen_lib.GenerateOutputFromTemplateString(backendTemplate, false, cfgMap, dstPath); err != nil {
		return skerr.Wrapf(err, "failed to write output")
	}
	return nil
}
