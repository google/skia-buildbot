package conversion

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	google3ParentName = "Google3"
)

var (
	// backendTemplate is the template used to generate the k8s YAML config file
	// for autoroll backends.
	//go:embed autoroll-be.yaml.template
	backendTemplate string

	// namespaceTemplate is the template used to generate the k8s YAML config
	// file for autoroll namespaces.
	//go:embed autoroll-ns.yaml.template
	namespaceTemplate string
)

// ConvertConfig converts the given roller config file to a Kubernetes config.
func ConvertConfig(ctx context.Context, cfgBytes []byte, relPath, dstDir string) error {
	if backendTemplate == "" {
		return skerr.Fmt("internal error; embedded template is empty")
	}

	// Decode the config file.
	var cfg config.Config
	if err := prototext.Unmarshal(cfgBytes, &cfg); err != nil {
		return skerr.Wrapf(err, "failed to parse roller config")
	}
	// Google3 uses a different type of backend.
	if cfg.ParentDisplayName == google3ParentName {
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
		// This causes the emitted config to be pretty-printed. It isn't needed
		// by the roller itself, but it's helpful for a human to debug issues
		// related to the config. Remove this if we start getting errors about
		// command lines being too long.
		Indent: "  ",
	}.Marshal(&cfg)
	if err != nil {
		return skerr.Wrapf(err, "Failed to encode roller config as text proto")
	}
	cfgFileBase64 := base64.StdEncoding.EncodeToString(b)
	cfgMap["configBase64"] = cfgFileBase64

	// Run kube-conf-gen to generate the backend config file.
	baseName, relDir := splitAndProcessPath(relPath)
	dstPath := filepath.Join(dstDir, relDir, fmt.Sprintf("autoroll-be-%s.yaml", strings.Split(baseName, ".")[0]))
	if err := kube_conf_gen_lib.GenerateOutputFromTemplateString(backendTemplate, false, cfgMap, dstPath); err != nil {
		return skerr.Wrapf(err, "failed to write output")
	}
	sklog.Infof("Wrote %s", dstPath)

	// Run kube-conf-gen to generate the namespace config file. Note that we'll
	// overwrite this file for every roller in the namespace, but that shouldn't
	// be a problem, since the generated files will be the same.
	namespace := strings.Split(cfg.ServiceAccount, "@")[0]
	dstNsPath := filepath.Join(dstDir, relDir, fmt.Sprintf("%s-ns.yaml", namespace))
	if err := kube_conf_gen_lib.GenerateOutputFromTemplateString(namespaceTemplate, false, cfgMap, dstNsPath); err != nil {
		return skerr.Wrapf(err, "failed to write output")
	}
	sklog.Infof("Wrote %s", dstNsPath)

	return nil
}

func splitAndProcessPath(path string) (string, string) {
	splitPath := strings.Split(path, string(filepath.Separator))
	baseName := splitPath[len(splitPath)-1]
	relDirParts := make([]string, 0, len(splitPath)-1)
	for _, part := range splitPath[:len(splitPath)-1] {
		if part != "generated" {
			relDirParts = append(relDirParts, part)
		}
	}
	relDir := strings.Join(relDirParts, string(filepath.Separator))
	return baseName, relDir
}
