package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/cd/go/cd"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/kube_conf_gen_lib"
	"go.skia.org/infra/task_driver/go/td"
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
	privacySandboxAndroidRepoURL := flag.String("privacy_sandbox_android_repo_url", "", "Repo URL for privacy sandbox on Android.")
	privacySandboxAndroidVersionsPath := flag.String("privacy_sandbox_android_versions_path", "", "Path to the file containing the versions of privacy sandbox on Android.")
	srcRepo := flag.String("source-repo", "https://skia.googlesource.com/skia-autoroll-internal-config.git", "URL of the repo which triggered this run.")
	srcCommit := flag.String("source-commit", "", "Commit hash which triggered this run.")
	louhiExecutionID := flag.String("louhi-execution-id", "", "Execution ID of the Louhi flow.")
	louhiPubsubProject := flag.String("louhi-pubsub-project", "", "GCP project used for sending Louhi pub/sub notifications.")

	flag.Parse()

	// We're using the task driver framework because it provides logging and
	// helpful insight into what's occurring as the program runs.
	fakeProjectId := ""
	fakeTaskId := ""
	fakeTaskName := ""
	output := "-"
	tdLocal := true
	ctx := td.StartRun(&fakeProjectId, &fakeTaskId, &fakeTaskName, &output, &tdLocal)
	defer td.EndRun(ctx)

	if backendTemplate == "" {
		td.Fatalf(ctx, "internal error; embedded template is empty.")
	}
	if *srcCommit == "" {
		td.Fatalf(ctx, "--source-commit is required.")
	}

	// Set up auth, load config variables.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail, gerrit.AuthScope)
	if err != nil {
		td.Fatal(ctx, err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	reg, err := config_vars.NewRegistry(ctx, chrome_branch.NewClient(client))
	if err != nil {
		td.Fatal(ctx, err)
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
			td.Fatal(ctx, err)
		}
	}
	b, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		td.Fatal(ctx, err)
	}
	sklog.Infof("Using variables: %s", string(b))

	// Walk through all files from the k8s-config repo. Read autoroll-related
	// config files.
	dst := gitiles.NewRepo("https://skia.googlesource.com/k8s-config.git", client)
	dstBaseCommit, err := dst.ResolveRef(ctx, git.DefaultRef)
	if err != nil {
		td.Fatal(ctx, err)
	}
	dstFiles, err := dst.ListFilesRecursiveAtRef(ctx, ".", dstBaseCommit)
	if err != nil {
		td.Fatal(ctx, err)
	}
	dstExistingContents := map[string][]byte{}
	for _, dstFile := range dstFiles {
		if strings.HasSuffix(dstFile, "-autoroll-ns.yaml") || (strings.HasPrefix(dstFile, "autoroll-be-") && strings.HasSuffix(dstFile, ".yaml")) {
			contents, err := dst.ReadFileAtRef(ctx, dstFile, dstBaseCommit)
			if err != nil {
				td.Fatal(ctx, err)
			}
			dstExistingContents[dstFile] = contents
		}
	}

	// Walk through the autoroller config repo. Create roller configs from
	// templates and convert roller configs to k8s configs.
	src := gitiles.NewRepo(*srcRepo, client)
	srcFiles, err := src.ListFilesRecursiveAtRef(ctx, ".", *srcCommit)
	if err != nil {
		td.Fatal(ctx, err)
	}
	generatedContents := map[string][]byte{}
	for _, srcFile := range srcFiles {
		if strings.HasSuffix(srcFile, ".cfg") {
			sklog.Infof("Converting %s", srcFile)
			cfgBytes, err := src.ReadFileAtRef(ctx, srcFile, *srcCommit)
			if err != nil {
				td.Fatalf(ctx, "failed to read roller config %s: %s", srcFile, err)
			}
			if err := convertConfig(ctx, cfgBytes, srcFile, generatedContents); err != nil {
				td.Fatalf(ctx, "failed to convert config %s: %s", srcFile, err)
			}
		} else if strings.HasSuffix(srcFile, ".tmpl") {
			sklog.Infof("Processing %s", srcFile)
			tmplBytes, err := src.ReadFileAtRef(ctx, srcFile, *srcCommit)
			if err != nil {
				td.Fatalf(ctx, "failed to read template file %s: %s", srcFile, err)
			}
			generatedConfigs, err := processTemplate(ctx, srcFile, string(tmplBytes), vars)
			if err != nil {
				td.Fatalf(ctx, "failed to process template file %s: %s", srcFile, err)
			}
			for path, cfgBytes := range generatedConfigs {
				if err := convertConfig(ctx, cfgBytes, path, generatedContents); err != nil {
					td.Fatalf(ctx, "failed to convert config %s: %s", srcFile, err)
				}
			}
		}
	}

	// Find the actual changes between the existing and the generated configs.
	changes := make(map[string]string, len(generatedContents))
	// First, "delete" all of the old contents, to ensure that we remove any
	// no-longer-generated rollers.
	for path := range dstExistingContents {
		changes[path] = ""
	}
	// Next, overwrite the old contents with the generated contents.
	for path, newContents := range generatedContents {
		changes[path] = string(newContents)
	}
	// Finally, remove any files which didn't actually change.
	for path, newContents := range generatedContents {
		oldContents, ok := dstExistingContents[path]
		if ok && bytes.Equal(oldContents, newContents) {
			delete(changes, path)
		}
	}

	// Upload a CL.
	if len(changes) > 0 {
		commitSubject := "Update autoroll k8s configs"
		if err := cd.UploadCL(ctx, changes, "https://skia.googlesource.com/k8s-config.git", dstBaseCommit, commitSubject, *srcRepo, *srcCommit, *louhiPubsubProject, *louhiExecutionID); err != nil {
			td.Fatalf(ctx, "Failed to create CL: %s", err)
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

func convertConfig(ctx context.Context, cfgBytes []byte, relPath string, generatedContents map[string][]byte) error {
	// Decode the config file.
	var cfg config.Config
	if err := prototext.Unmarshal(cfgBytes, &cfg); err != nil {
		return skerr.Wrapf(err, "failed to parse roller config: %s", string(cfgBytes))
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
	relDir, baseName := path.Split(relPath)
	dstCfgPath := path.Join(relDir, fmt.Sprintf("autoroll-be-%s.yaml", strings.Split(baseName, ".")[0]))
	rollerCfg, err := kube_conf_gen_lib.GenerateOutputFromTemplateString(backendTemplate, false, cfgMap)
	if err != nil {
		return skerr.Wrapf(err, "failed to write output")
	}
	generatedContents[dstCfgPath] = rollerCfg

	// Run kube-conf-gen to generate the namespace config file. Note that we'll
	// overwrite this file for every roller in the namespace, but that shouldn't
	// be a problem, since the generated files will be the same.
	namespace := strings.Split(cfg.ServiceAccount, "@")[0]
	dstNsPath := path.Join(relDir, fmt.Sprintf("%s-ns.yaml", namespace))
	nsCfg, err := kube_conf_gen_lib.GenerateOutputFromTemplateString(namespaceTemplate, false, cfgMap)
	if err != nil {
		return skerr.Wrapf(err, "failed to write output")
	}
	generatedContents[dstNsPath] = nsCfg

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
