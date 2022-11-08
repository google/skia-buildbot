package main

import (
	"bytes"
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
	"sync"
	"text/template"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/cd/go/cd"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/kube_conf_gen_lib"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
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

	// protoMarshalOptions are used when writing configs in text proto format.
	protoMarshalOptions = prototext.MarshalOptions{
		Multiline: true,
	}
	// funcMap is used for executing templates.
	funcMap = template.FuncMap{
		"map": makeMap,
	}
)

func main() {
	// Flags.
	src := flag.String("src", "", "Source directory.")
	dst := flag.String("dst", "", "Destination directory. Outputs will mimic the structure of the source.")
	privacySandboxAndroidRepoURL := flag.String("privacy_sandbox_android_repo_url", "", "Repo URL for privacy sandbox on Android.")
	privacySandboxAndroidVersionsPath := flag.String("privacy_sandbox_android_versions_path", "", "Path to the file containing the versions of privacy sandbox on Android.")
	createCL := flag.Bool("create-cl", false, "If true, creates a CL if any changes were made.")
	srcRepo := flag.String("source-repo", "", "URL of the repo which triggered this run.")
	srcCommit := flag.String("source-commit", "", "Commit hash which triggered this run.")
	louhiExecutionID := flag.String("louhi-execution-id", "", "Execution ID of the Louhi flow.")
	louhiPubsubProject := flag.String("louhi-pubsub-project", "", "GCP project used for sending Louhi pub/sub notifications.")

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

	// Set up auth, load config variables.
	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, gerrit.AuthScope)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	reg, err := config_vars.NewRegistry(ctx, chrome_branch.NewClient(client))
	if err != nil {
		sklog.Fatal(err)
	}
	vars := &TemplateVars{
		Vars: reg.Vars(),
	}

	// Load the privacy sandbox versions for each of the active milestones.
	if *privacySandboxAndroidRepoURL != "" && *privacySandboxAndroidVersionsPath != "" {
		var eg errgroup.Group
		repo := gitiles.NewRepo(*privacySandboxAndroidRepoURL, client)
		var mtx sync.Mutex
		milestones := append(vars.Branches.ActiveMilestones, vars.Branches.Chromium.Main)
		for _, m := range milestones {
			m := m // https://golang.org/doc/faq#closures_and_goroutines
			eg.Go(func() error {
				branchName := fmt.Sprintf("m%d", m.Milestone)
				ref := fmt.Sprintf("refs/heads/chromium/%d", m.Number)
				if m.Number == 0 {
					branchName = "main"
					ref = "refs/heads/main"
				}
				sklog.Infof("Reading privacy sandbox versions at milestone: %+v", m)
				contents, err := repo.ReadFileAtRef(ctx, *privacySandboxAndroidVersionsPath, ref)
				if err != nil {
					if strings.Contains(err.Error(), "NOT_FOUND") {
						sklog.Warningf("%s not found in %s", *privacySandboxAndroidVersionsPath, ref)
						return nil
					}
					return skerr.Wrapf(err, "failed to load privacy sandbox version for %s", ref)
				}
				var psVersions []*PrivacySandboxVersion
				if err := json.Unmarshal(contents, &psVersions); err != nil {
					return skerr.Wrapf(err, "failed to parse privacy sandbox version for %s from %s", ref, string(contents))
				}
				for _, v := range psVersions {
					v.BranchName = branchName
					v.Ref = ref
				}
				mtx.Lock()
				defer mtx.Unlock()
				vars.PrivacySandboxVersions = append(vars.PrivacySandboxVersions, psVersions...)
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			sklog.Fatal(err)
		}
	}
	b, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Using variables: %s", string(b))

	// Walk through the autoroller config directory. Create roller configs from
	// templates and convert roller configs to k8s configs.
	fsys := os.DirFS(*src)
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".cfg") {
			srcPath := filepath.Join(*src, path)
			sklog.Infof("Converting %s", srcPath)
			cfgBytes, err := ioutil.ReadFile(srcPath)
			if err != nil {
				return skerr.Wrapf(err, "failed to read roller config %s", srcPath)
			}

			if err := convertConfig(ctx, cfgBytes, path, *dst); err != nil {
				return skerr.Wrapf(err, "failed to convert config %s", path)
			}
		} else if strings.HasSuffix(d.Name(), ".tmpl") {
			tmplPath := filepath.Join(*src, path)
			sklog.Infof("Processing %s", tmplPath)
			tmplContents, err := ioutil.ReadFile(tmplPath)
			if err != nil {
				return skerr.Wrapf(err, "failed to read template file %s", tmplPath)
			}
			generatedConfigs, err := processTemplate(ctx, path, string(tmplContents), vars)
			if err != nil {
				return skerr.Wrapf(err, "failed to process template file %s", path)
			}
			for path, cfgBytes := range generatedConfigs {
				if err := convertConfig(ctx, cfgBytes, path, *dst); err != nil {
					return skerr.Wrapf(err, "failed to convert config %s", path)
				}
			}
		}
		return nil
	}); err != nil {
		sklog.Fatalf("Failed to read configs: %s", err)
	}

	// Upload a CL.
	if *createCL {
		commitSubject := "Update autoroll k8s configs"
		if err := cd.MaybeUploadCL(ctx, *dst, commitSubject, *srcRepo, *srcCommit, *louhiPubsubProject, *louhiExecutionID); err != nil {
			sklog.Fatalf("Failed to create CL: %s", err)
		}
	}
}

// PrivacySandboxVersion tracks a single version of the privacy sandbox.
type PrivacySandboxVersion struct {
	BranchName    string `json:"BranchName"`
	Ref           string `json:"Ref"`
	PylFile       string `json:"PylFile"`
	PylTargetPath string `json:"PylTargetPath"`
	CipdPackage   string `json:"CipdPackage"`
	CipdTag       string `json:"CipdTag"`
}

type TemplateVars struct {
	*config_vars.Vars
	PrivacySandboxVersions []*PrivacySandboxVersion
}

func convertConfig(ctx context.Context, cfgBytes []byte, relPath, dstDir string) error {
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
	relDir, baseName := filepath.Split(relPath)
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

// processTemplate converts a single template into at least one config.
func processTemplate(ctx context.Context, srcPath, tmplContents string, vars *TemplateVars) (map[string][]byte, error) {
	// Read and execute the template.
	tmpl, err := template.New(filepath.Base(srcPath)).Funcs(funcMap).Parse(tmplContents)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse template file %q", srcPath)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, skerr.Wrapf(err, "failed to execute template file %q", srcPath)
	}

	// Parse the template as a list of Configs.
	var configs config.Configs
	if err := prototext.Unmarshal(buf.Bytes(), &configs); err != nil {
		return nil, skerr.Wrapf(err, "failed to parse config proto from template file %q", srcPath)
	}
	sklog.Infof("  Found %d configs in %s", len(configs.Config), srcPath)

	// Split off the template file name and the "templates" directory name.
	srcPathSplit := []string{}
	for _, elem := range strings.Split(srcPath, string(filepath.Separator)) {
		if !strings.HasSuffix(elem, ".tmpl") && elem != "templates" {
			srcPathSplit = append(srcPathSplit, elem)
		}
	}
	srcRelPath := filepath.Join(srcPathSplit...)

	changes := make(map[string][]byte, len(configs.Config))
	for _, cfg := range configs.Config {
		encBytes, err := protoMarshalOptions.Marshal(cfg)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to encode config from %q", srcPath)
		}
		changes[filepath.Join(srcRelPath, cfg.RollerName+".cfg")] = encBytes
	}
	return changes, nil
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
