package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/autoroll/go/config/conversion"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
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
	var vars conversion.TemplateVars
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
		generatedConfigs, err := ProcessTemplate(tmplPath, &vars)
		if err != nil {
			return skerr.Wrapf(err, "failed to process template file %s", tmplPath)
		}
		for path, cfgBytes := range generatedConfigs {
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return skerr.Wrapf(err, "failed to create dir %s", dir)
			}
			if err := ioutil.WriteFile(path, cfgBytes, os.ModePerm); err != nil {
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

	vars, err := conversion.CreateTemplateVars(ctx, client, privacySandboxAndroidRepoURL, privacySandboxAndroidVersionsPath)
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

// ProcessTemplate converts a single template into at least one config.
func ProcessTemplate(srcPath string, vars *conversion.TemplateVars) (map[string][]byte, error) {
	// Read and execute the template.
	tmplContents, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read template file %s", srcPath)
	}
	tmpl, err := template.New(filepath.Base(srcPath)).Funcs(conversion.FuncMap).Parse(string(tmplContents))
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
