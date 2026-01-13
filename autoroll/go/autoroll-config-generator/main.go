package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
)

var (
	// FuncMap is used for executing templates.
	FuncMap = template.FuncMap{
		"map":                         makeMap,
		"list":                        makeList,
		"sanitize":                    sanitize,
		"truncateAndSanitizeRollerID": truncateAndSanitizeRollerID,
	}
)

func main() {
	const (
		flagTmplFlagsFile                     = "vars-file"
		flagDir                               = "in"
		flagPrivacySandboxAndroidRepoURL      = "privacy-sandbox-android-repo-url"
		flagPrivacySandboxAndroidVersionsPath = "privacy-sandbox-android-versions-path"
	)
	app := &cli.App{
		Name:        "autoroll-config-generator",
		Description: `autoroll-config-generator generates autoroll configs from templates.`,
		Commands: []*cli.Command{
			{
				Name:        "generate",
				Description: "Generate autoroll configs.",
				Usage:       "generate <options>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     flagTmplFlagsFile,
						Usage:    "File containing the template input variables.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagDir,
						Usage:    "Directory in which to search for templates.",
						Required: true,
					},
				},
				Action: func(ctx *cli.Context) error {
					return generate(ctx.Context, ctx.String(flagTmplFlagsFile), ctx.String(flagDir))
				},
			},
			{
				Name:        "update-inputs",
				Description: "Update the saved input variables.",
				Usage:       "update-inputs <options>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     flagTmplFlagsFile,
						Usage:    "File containing the template input variables.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagPrivacySandboxAndroidRepoURL,
						Usage:    "Repo URL for privacy sandbox on Android.",
						Required: true,
					},
					&cli.StringFlag{
						Name:     flagPrivacySandboxAndroidVersionsPath,
						Usage:    "Path to the file containing the versions of privacy sandbox on Android.",
						Required: true,
					},
				},
				Action: func(ctx *cli.Context) error {
					return updateInputs(ctx.Context, ctx.String(flagTmplFlagsFile), ctx.String(flagPrivacySandboxAndroidRepoURL), ctx.String(flagPrivacySandboxAndroidVersionsPath))
				},
			},
		},
		Usage: "autoroll-config-generator <subcommand>",
	}
	if err := app.RunContext(context.Background(), os.Args); err != nil {
		sklog.Fatal(err)
	}
}

func generate(ctx context.Context, tmplVarsFile, dir string) error {
	// Load config variables.
	var vars templateVars
	if err := util.WithReadFile(tmplVarsFile, func(f io.Reader) error {
		return json.NewDecoder(f).Decode(&vars)
	}); err != nil {
		return skerr.Wrap(err)
	}

	// Walk through the directory looking for templates and previously-generated
	// configs.
	templates := []string{}
	oldConfigs := []string{}
	fsys := os.DirFS(dir)
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tmpl") {
			templates = append(templates, path)
		}
		if strings.Contains(path, string(filepath.Separator)+"generated"+string(filepath.Separator)) {
			oldConfigs = append(oldConfigs, path)
		}
		return nil
	}); err != nil {
		return skerr.Wrap(err)
	}
	for _, oldConfig := range oldConfigs {
		fmt.Printf("Deleting old config %s\n", oldConfig)
		if err := os.Remove(oldConfig); err != nil {
			return skerr.Wrapf(err, "failed to remove old config %s", oldConfig)
		}
	}
	for _, tmplPath := range templates {
		fmt.Printf("Processing %s\n", tmplPath)
		generatedConfigs, err := processTemplate(tmplPath, &vars)
		if err != nil {
			return skerr.Wrapf(err, "failed to process template file %s", tmplPath)
		}
		for path, cfgBytes := range generatedConfigs {
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return skerr.Wrapf(err, "failed to create dir %s", dir)
			}
			if err := os.WriteFile(path, cfgBytes, 0644); err != nil {
				return skerr.Wrapf(err, "failed to write %s", path)
			}
		}
	}
	return nil
}

func updateInputs(ctx context.Context, tmplVarsFile, privacySandboxAndroidRepoURL, privacySandboxAndroidVersionsPath string) error {
	// Set up auth, load config variables.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	vars, err := createTemplateVars(ctx, client, privacySandboxAndroidRepoURL, privacySandboxAndroidVersionsPath)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Write the template variables to the destination file.
	return util.WithWriteFile(tmplVarsFile, func(f io.Writer) error {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		return enc.Encode(vars)
	})
}

var rollerNameRegex = regexp.MustCompile(`(?m)^\s*roller_name:\s*"(\S+)"`)

// processTemplate converts a single template into at least one config.
func processTemplate(srcPath string, vars *templateVars) (map[string][]byte, error) {
	// Read and execute the template.
	tmplContents, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read template file %s", srcPath)
	}
	tmpl, err := template.New(filepath.Base(srcPath)).Funcs(FuncMap).Parse(string(tmplContents))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse template file %q", srcPath)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, skerr.Wrapf(err, "failed to execute template file %q", srcPath)
	}

	// Split the resulting contents into one file per config.
	configsBytes := splitConfigs(buf.Bytes())
	fmt.Printf("Found %d configs in %s\n", len(configsBytes), srcPath)

	// Split off the template file name and the "templates" directory name.
	srcPathSplit := []string{}
	for _, elem := range strings.Split(srcPath, string(filepath.Separator)) {
		if elem == "templates" {
			srcPathSplit = append(srcPathSplit, "generated")
		} else if !strings.HasSuffix(elem, ".tmpl") {
			srcPathSplit = append(srcPathSplit, elem)
		}
	}
	srcRelPath := filepath.Join(srcPathSplit...)

	// Find the names of the rollers.
	changes := make(map[string][]byte, len(configsBytes))
	for _, configBytes := range configsBytes {
		matches := rollerNameRegex.FindSubmatch(configBytes)
		if len(matches) != 2 {
			return nil, skerr.Fmt("failed to find roller_name in %s:\n%s", srcPath, string(configBytes))
		}
		dstFile := filepath.Join(srcRelPath, string(matches[1])+".cfg")
		if strings.HasPrefix(srcPath, string(filepath.Separator)) && !strings.HasPrefix(dstFile, string(filepath.Separator)) {
			dstFile = string(filepath.Separator) + dstFile
		}
		if _, ok := changes[dstFile]; ok {
			return nil, skerr.Fmt("multiple templates produced %s", dstFile)
		}
		configBytes, err = parser.FormatWithConfig(configBytes, parser.Config{
			ExpandAllChildren:                      true,
			SkipAllColons:                          true,
			SortFieldsByFieldName:                  false,
			SortRepeatedFieldsByContent:            false,
			SortRepeatedFieldsBySubfield:           nil,
			RemoveDuplicateValuesForRepeatedFields: false,
			AllowTripleQuotedStrings:               false,
			WrapStringsAtColumn:                    0,
			WrapHTMLStrings:                        false,
			WrapStringsAfterNewlines:               false,
			PreserveAngleBrackets:                  false,
			SmartQuotes:                            false,
		})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		changes[dstFile] = configBytes
	}
	return changes, nil
}

var configStartRegex = regexp.MustCompile(`(?m)^\s*config\s*\{`)
var configEndRegex = regexp.MustCompile(`\s*\}\s*$`)

// splitConfigs takes the template results containing multiple config
// definitions and splits into individual config definitions.
func splitConfigs(allConfigs []byte) [][]byte {
	splitIndexes := configStartRegex.FindAllIndex(allConfigs, -1)
	configsBytes := make([][]byte, 0, len(splitIndexes))
	for i, splitIndex := range splitIndexes {
		startIndex := splitIndex[1]
		endIndex := len(allConfigs)
		if i < len(splitIndexes)-1 {
			endIndex = splitIndexes[i+1][0]
		}
		configBytes := allConfigs[startIndex:endIndex]
		configBytes = configEndRegex.ReplaceAll(configBytes, []byte(""))
		configsBytes = append(configsBytes, configBytes)
	}
	return configsBytes
}

// createTemplateVars reads data from multiple sources to produce variables used
// as input to templates.
func createTemplateVars(ctx context.Context, client *http.Client, privacySandboxAndroidRepoURL, privacySandboxAndroidVersionsPath string) (*templateVars, error) {
	reg, err := config_vars.NewRegistry(ctx, chrome_branch.NewClient(client))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	vars := &templateVars{
		Vars: reg.Vars(),
	}

	// Load the privacy sandbox versions for each of the active milestones.
	if privacySandboxAndroidRepoURL != "" && privacySandboxAndroidVersionsPath != "" {
		var eg errgroup.Group
		repo := gitiles.NewRepo(privacySandboxAndroidRepoURL, client)
		var mtx sync.Mutex
		milestones := vars.Branches.ActiveMilestones
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
				var psVersions []*privacySandboxVersion
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
		sort.Sort(privacySandboxVersionSlice(vars.PrivacySandboxVersions))
	}

	return vars, nil
}

// privacySandboxVersion tracks a single version of the privacy sandbox.
type privacySandboxVersion struct {
	BranchName    string `json:"BranchName"`
	Ref           string `json:"Ref"`
	Bucket        string `json:"Bucket"`
	PylFile       string `json:"PylFile"`
	PylTargetPath string `json:"PylTargetPath"`
	CipdPackage   string `json:"CipdPackage"`
	CipdTag       string `json:"CipdTag"`
}

// privacySandboxVersionSlice implements sort.Interface.
type privacySandboxVersionSlice []*privacySandboxVersion

// Len implements sort.Interface.
func (s privacySandboxVersionSlice) Len() int {
	return len(s)
}

func sortHelper(a, b string) (bool, bool) {
	if a != b {
		return true, a < b
	}
	return false, false
}

// Less implements sort.Interface.
func (s privacySandboxVersionSlice) Less(i, j int) bool {
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
func (s privacySandboxVersionSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type templateVars struct {
	*config_vars.Vars
	PrivacySandboxVersions []*privacySandboxVersion
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

func truncateAndSanitizeRollerID(v string) string {
	if len(v) < config.MaxRollerNameLength {
		return sanitize(v)
	}
	return sanitize(v[:config.MaxRollerNameLength])
}
