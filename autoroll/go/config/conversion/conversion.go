package conversion

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/kube_conf_gen_lib"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
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
	// FuncMap is used for executing templates.
	FuncMap = template.FuncMap{
		"map":      makeMap,
		"list":     makeList,
		"sanitize": sanitize,
	}
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

// CreateTemplateVars reads data from multiple sources to produce variables used
// as input to templates.
func CreateTemplateVars(ctx context.Context, client *http.Client, privacySandboxAndroidRepoURL, privacySandboxAndroidVersionsPath string) (*TemplateVars, error) {
	reg, err := config_vars.NewRegistry(ctx, chrome_branch.NewClient(client))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	vars := &TemplateVars{
		Vars: reg.Vars(),
	}

	// Load the privacy sandbox versions for each of the active milestones.
	if privacySandboxAndroidRepoURL != "" && privacySandboxAndroidVersionsPath != "" {
		var eg errgroup.Group
		repo := gitiles.NewRepo(privacySandboxAndroidRepoURL, client)
		var mtx sync.Mutex
		milestones := append(vars.Branches.ActiveMilestones, vars.Branches.Chromium.Main)
		for _, m := range milestones {
			m := m // https://golang.org/doc/faq#closures_and_goroutines
			eg.Go(func() error {
				branchName := fmt.Sprintf("m%d", m.Milestone)
				ref := fmt.Sprintf("refs/heads/chromium/%d", m.Number)
				bucket := fmt.Sprintf("luci.chrome-m%d.try", m.Milestone)
				if m.Number == 0 {
					branchName = "main"
					ref = "refs/heads/main"
					bucket = "luci.chrome.try"
				}
				sklog.Infof("Reading privacy sandbox versions at milestone: %+v", m)
				contents, err := repo.ReadFileAtRef(ctx, privacySandboxAndroidVersionsPath, ref)
				if err != nil {
					if strings.Contains(err.Error(), "NOT_FOUND") {
						sklog.Warningf("%s not found in %s", privacySandboxAndroidVersionsPath, ref)
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
					v.Bucket = bucket
				}
				mtx.Lock()
				defer mtx.Unlock()
				vars.PrivacySandboxVersions = append(vars.PrivacySandboxVersions, psVersions...)
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return nil, skerr.Wrap(err)
		}
		sort.Sort(PrivacySandboxVersionSlice(vars.PrivacySandboxVersions))
	}

	return vars, nil
}

// ProcessTemplate converts a single template into at least one config.
func ProcessTemplate(ctx context.Context, client *http.Client, srcPath, tmplContents string, vars *TemplateVars, checkGCSArtifacts bool) (map[string][]byte, error) {
	// Read and execute the template.
	tmpl, err := template.New(filepath.Base(srcPath)).Funcs(FuncMap).Parse(tmplContents)
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

	// Filter out any rollers whose required GCS artifacts do not exist.
	filteredConfigs := make([]*config.Config, 0, len(configs.Config))
	for _, cfg := range configs.Config {
		missing := false
		if checkGCSArtifacts {
			var err error
			missing, err = gcsArtifactIsMissing(ctx, client, cfg)
			if err != nil {
				return nil, skerr.Wrapf(err, "failed to check whether GCS artifact exists for %s (from %s)", cfg.RollerName, srcPath)
			}
		}
		if missing {
			sklog.Warningf("Skipping roller %s; required GCS artifact does not exist.", cfg.RollerName)
		} else {
			filteredConfigs = append(filteredConfigs, cfg)
		}
	}

	// Split off the template file name and the "templates" directory name.
	srcPathSplit := []string{}
	for _, elem := range strings.Split(srcPath, string(filepath.Separator)) {
		if !strings.HasSuffix(elem, ".tmpl") && elem != "templates" {
			srcPathSplit = append(srcPathSplit, elem)
		}
	}
	srcRelPath := filepath.Join(srcPathSplit...)

	changes := make(map[string][]byte, len(filteredConfigs))
	for _, cfg := range filteredConfigs {
		encBytes, err := protoMarshalOptions.Marshal(cfg)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to encode config from %q", srcPath)
		}
		changes[filepath.Join(srcRelPath, cfg.RollerName+".cfg")] = encBytes
	}
	return changes, nil
}

func gcsArtifactIsMissing(ctx context.Context, client *http.Client, cfg *config.Config) (bool, error) {
	pcrm := cfg.GetParentChildRepoManager()
	if pcrm == nil {
		return false, nil
	}
	semVerGCSChild := pcrm.GetSemverGcsChild()
	if semVerGCSChild == nil {
		return false, nil
	}
	gcsChild := semVerGCSChild.Gcs
	if gcsChild == nil {
		// This shouldn't happen with a valid config.
		return false, nil
	}
	regex, err := regexp.Compile(semVerGCSChild.VersionRegex)
	if err != nil {
		return false, skerr.Wrapf(err, "failed compiling regex for %s", cfg.RollerName)
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return false, skerr.Wrap(err)
	}
	gcsClient := gcsclient.New(storageClient, gcsChild.GcsBucket)
	missing := true
	if err := gcsClient.AllFilesInDirectory(ctx, gcsChild.GcsPath, func(item *storage.ObjectAttrs) error {
		if regex.MatchString(path.Base(item.Name)) {
			missing = false
			return iterator.Done
		}
		return nil
	}); err != nil && err != iterator.Done {
		return false, skerr.Wrapf(err, "failed searching %s/%s", gcsChild.GcsBucket, gcsChild.GcsPath)
	}
	return missing, nil
}

// PrivacySandboxVersion tracks a single version of the privacy sandbox.
type PrivacySandboxVersion struct {
	BranchName    string `json:"BranchName"`
	Ref           string `json:"Ref"`
	Bucket        string `json:"Bucket"`
	PylFile       string `json:"PylFile"`
	PylTargetPath string `json:"PylTargetPath"`
	CipdPackage   string `json:"CipdPackage"`
	CipdTag       string `json:"CipdTag"`
}

// PrivacySandboxVersionSlice implements sort.Interface.
type PrivacySandboxVersionSlice []*PrivacySandboxVersion

// Len implements sort.Interface.
func (s PrivacySandboxVersionSlice) Len() int {
	return len(s)
}

func sortHelper(a, b string) (bool, bool) {
	if a != b {
		return true, a < b
	}
	return false, false
}

// Less implements sort.Interface.
func (s PrivacySandboxVersionSlice) Less(i, j int) bool {
	a := s[i]
	b := s[j]
	if diff, less := sortHelper(a.BranchName, b.BranchName); diff {
		return less
	}
	if diff, less := sortHelper(a.Ref, b.Ref); diff {
		return less
	}
	if diff, less := sortHelper(a.Bucket, b.Bucket); diff {
		return less
	}
	if diff, less := sortHelper(a.CipdPackage, b.CipdPackage); diff {
		return less
	}
	if diff, less := sortHelper(a.CipdTag, b.CipdTag); diff {
		return less
	}
	if diff, less := sortHelper(a.PylFile, b.PylFile); diff {
		return less
	}
	if diff, less := sortHelper(a.PylTargetPath, b.PylTargetPath); diff {
		return less
	}
	return false
}

// Swap implements sort.Interface.
func (s PrivacySandboxVersionSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type TemplateVars struct {
	*config_vars.Vars
	PrivacySandboxVersions []*PrivacySandboxVersion
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

func makeList(args ...interface{}) []interface{} {
	return args
}

func sanitize(v string) string {
	re1 := regexp.MustCompile(`[^a-zA-Z0-9-]+`)
	v = re1.ReplaceAllString(v, "-")
	re2 := regexp.MustCompile(`--+`)
	v = re2.ReplaceAllString(v, "-")
	return v
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
