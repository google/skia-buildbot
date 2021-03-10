// Package frontend provides a Gazelle extension for generating Bazel build targets for custom web
// elements used to build the web UIs of most Skia Infrastructure applications.
//
// This extension looks for custom elements defined in directories that conform to the
// //<app-name>/modules/<custom-element-sk> naming convention, and will generate/update BUILD.bazel
// files inside said directories with the following kinds of build targets:
//
//  - ts_library
//  - karma_test
//  - sass_library
//  - sk_element
//  - sk_page
//  - sk_element_demo_page_server
//  - sk_element_puppeteer_test
//
// This extension also looks for *.ts, *_test.ts and *.scss files outside of said directories, and
// generates ts_library, karma_test/nodejs_mocha_test and sass_library targets for those files,
// respectively.
//
// A Gazelle extension is essentially a go_library with a function named NewLanguage that provides
// an implementation of the language.Language interface. This interface provides hooks for
// generating rules, parsing configuration directives, and resolving imports to Bazel labels.
//
// Docs on Gazelle extensions: https://github.com/bazelbuild/bazel-gazelle/blob/master/extend.rst.
package frontend

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"go.skia.org/infra/go/util"
)

// targetDirectories is a set of known good directories for which we can currently generate valid
// build targets. This Gazelle extension will not generate build targets for any other directories
// in the repository.
//
// The key of this map indicates whether to recurse into the directory.
//
// TODO(lovisolo): Delete.
var targetDirectories = map[string]bool{
	"infra-sk/modules":                       false,
	"infra-sk/modules/ElementSk":             false,
	"infra-sk/modules/login-sk":              false,
	"infra-sk/modules/page_object":           false,
	"infra-sk/modules/paramset-sk":           false,
	"infra-sk/modules/query-values-sk":       false,
	"infra-sk/modules/query-sk":              false,
	"infra-sk/modules/sort-sk":               false,
	"infra-sk/modules/theme-chooser-sk":      false,
	"machine/modules/json":                   false,
	"machine/modules/machine-server-sk":      false,
	"new_element/modules/example-control-sk": false,
	"perf/modules":                           true,
	"perf/pages":                             false,
	"puppeteer-tests":                        false,
}

func isTargetDirectory(dir string) bool {
	for targetDir, recursive := range targetDirectories {
		if dir == targetDir || (recursive && strings.HasPrefix(dir, targetDir+"/")) {
			return true
		}
	}
	return false
}

const (
	// frontendExtName is the extension name passed to Gazelle.
	frontendExtName = "frontend"

	// Namespace under which NPM modules are exposed. This must match the WORKSPACE file.
	npmBazelNamespace = "infra-sk_npm"
)

//////////////////////////////////////
// Command-line flags and utilities //
//////////////////////////////////////

// frontendFlags contains the values of this extension's command-line flags.
//
// This struct is instantiated/populated in frontendConfigurer.RegisterFlags, and can be retrieved
// from a config.Config struct via the getFrontendFlags function.
type frontendFlags struct {
	apps string
}

// getFrontendFlags extracts the frontendFlags from a config.Config struct.
func getFrontendFlags(c *config.Config) *frontendFlags {
	ext := c.Exts[frontendExtName]
	if ext == nil {
		return nil
	}
	return ext.(*frontendFlags)
}

////////////////
// Configurer //
////////////////

// frontendConfigurer implements the config.Configurer interface.
type frontendConfigurer struct{}

// RegisterFlags implements the config.Configurer interface.
func (fc *frontendConfigurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	ff := &frontendFlags{}
	c.Exts[frontendExtName] = ff

	fs.StringVar(&ff.apps, "frontend-apps", "", "Comma-separated list of Skia Infra apps for which to generate front-end BUILD files (no-op if empty).")
}

// CheckFlags implements the config.Configurer interface.
func (fc *frontendConfigurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}

// KnownDirectives implements the config.Configurer interface.
func (fc *frontendConfigurer) KnownDirectives() []string {
	return []string{"karma_test", "nodejs_test", "sass_library", "sk_demo_page_server", "sk_element", "sk_element_puppeteer_test", "sk_page", "ts_library"}
}

// Configure implements the config.Configurer interface.
func (fc *frontendConfigurer) Configure(c *config.Config, rel string, f *rule.File) {
}

var _ config.Configurer = &frontendConfigurer{}

//////////////
// Resolver //
//////////////

// frontendResolver implements the resolve.Resolver interface.
type frontendResolver struct {
	npmPackages map[string]bool // Set of dependencies and devDependencies in the package.json file.

	sassImportsToDeps map[string]map[ruleKindAndLabel]bool // Maps Sass imports to rules that can be added as dependencies to provide those imports.
	tsImportsToDeps   map[string]map[ruleKindAndLabel]bool // Maps TypeScript imports to rules that can be added as dependencies to provide those imports.
}

type ruleKindAndLabel struct {
	kind  string // Rule kind, e.g. "ts_library", "sass_library", "sk_element", etc.
	label label.Label
}

var noRuleKindAndLabel = ruleKindAndLabel{}

func (fr *frontendResolver) indexImportsProvidedByRule(lang string, importPaths []string, ruleKind string, ruleLabel label.Label) {
	if lang != "sass" && lang != "ts" {
		log.Panicf("Unknown language: %q.", lang)
	}

	if fr.sassImportsToDeps == nil {
		fr.sassImportsToDeps = map[string]map[ruleKindAndLabel]bool{}
	}
	if fr.tsImportsToDeps == nil {
		fr.tsImportsToDeps = map[string]map[ruleKindAndLabel]bool{}
	}

	importsToDeps := fr.sassImportsToDeps
	if lang == "ts" {
		importsToDeps = fr.tsImportsToDeps
	}

	for _, importPath := range importPaths {
		if importsToDeps[importPath] == nil {
			importsToDeps[importPath] = map[ruleKindAndLabel]bool{}
		}
		rkal := ruleKindAndLabel{kind: ruleKind, label: ruleLabel}
		importsToDeps[importPath][rkal] = true
	}
}

func (fr *frontendResolver) findRuleThatProvidesImport(lang string, importPath string, fromRuleKind string, fromRuleLabel label.Label) ruleKindAndLabel {
	if lang != "sass" && lang != "ts" {
		log.Panicf("Unknown language: %q.", lang)
	}

	importsToDeps := fr.sassImportsToDeps
	if lang == "ts" {
		importsToDeps = fr.tsImportsToDeps
	}

	var candidates []ruleKindAndLabel
	if importsToDeps[importPath] != nil {
		for c := range importsToDeps[importPath] {
			candidates = append(candidates, c)
		}
	}

	if len(candidates) == 0 {
		log.Printf("Could not find any rules that satisfy import %q from %s (%s)", importPath, fromRuleLabel, fromRuleKind)
		return noRuleKindAndLabel
	}

	if len(candidates) > 1 {
		log.Printf("Multiple rules satisfy import %q from %s (%s): %s (%s), %s (%s)", importPath, fromRuleLabel, fromRuleKind, candidates[0].label, candidates[0].kind, candidates[1].label, candidates[1].kind)
		return noRuleKindAndLabel
	}

	return candidates[0]
}

// Name implements the resolve.Resolver interface.
func (fr *frontendResolver) Name() string {
	return frontendExtName
}

// Imports implements the resolve.Resolver interface.
func (fr *frontendResolver) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	// fmt.Printf("EXTRACTING IMPORTS FROM %s RULE %s:%s\n", r.Kind(), f.Pkg, r.Name())

	ruleLabel := label.New(c.RepoName, f.Pkg, r.Name())

	switch r.Kind() {
	case "ts_library":
		importPaths := extractTypeScriptImportsProvidedByRule(f.Pkg, r, "srcs")
		// fmt.Printf("TS_LIBRARY IMPORTS: %v\n", importPaths)
		fr.indexImportsProvidedByRule("ts", importPaths, r.Kind(), ruleLabel)
	case "sass_library":
		importPaths := extractSassImportsProvidedByRule(f.Pkg, r, "srcs")
		// fmt.Printf("SASS_LIBRARY IMPORTS: %v\n", importPaths)
		fr.indexImportsProvidedByRule("sass", importPaths, r.Kind(), ruleLabel)
	case "sk_element":
		tsImportPaths := extractTypeScriptImportsProvidedByRule(f.Pkg, r, "ts_srcs")
		sassImportPaths := extractSassImportsProvidedByRule(f.Pkg, r, "sass_srcs")
		// fmt.Printf("SK_ELEMENT TS IMPORTS: %v\n", tsImportPaths)
		// fmt.Printf("SK_ELEMENT SASS IMPORTS: %v\n", sassImportPaths)
		fr.indexImportsProvidedByRule("ts", tsImportPaths, r.Kind(), ruleLabel)
		fr.indexImportsProvidedByRule("sass", sassImportPaths, r.Kind(), ruleLabel)
	}

	return []resolve.ImportSpec{}
}

func extractTypeScriptImportsProvidedByRule(pkg string, r *rule.Rule, srcsAttr string) []string {
	var importPaths []string
	for _, src := range r.AttrStrings(srcsAttr) {
		if !strings.HasSuffix(src, ".ts") {
			log.Printf("Rule %s of kind %s contains a non-TypeScript file in its %s attribute: %s", label.New("", pkg, r.Name()).String(), r.Kind(), srcsAttr, src)
			continue
		}

		importPaths = append(importPaths, path.Join(pkg, strings.TrimSuffix(src, path.Ext(src))))

		// An index.ts file may also be imported as its parent folder's "main" module:
		//
		//     // The two following imports are equivalent.
		//     import 'path/to/module/index';
		//     import 'path/to/module';
		//
		// Reference:
		// https://www.typescriptlang.org/docs/handbook/module-resolution.html#how-typescript-resolves-modules.
		if src == "index.ts" {
			importPaths = append(importPaths, pkg)
		}
	}
	return importPaths
}

func extractSassImportsProvidedByRule(pkg string, r *rule.Rule, srcsAttr string) []string {
	var importPaths []string
	for _, src := range r.AttrStrings(srcsAttr) {
		if !strings.HasSuffix(src, ".scss") {
			log.Printf("Rule %s of kind %s contains a non-Sass file in its %s attribute: %s", label.New("", pkg, r.Name()).String(), r.Kind(), srcsAttr, src)
			continue
		}
		importPaths = append(importPaths, path.Join(pkg, strings.TrimSuffix(src, path.Ext(src))))
	}
	return importPaths
}

// Embeds implements the resolve.Resolver interface.
func (fr *frontendResolver) Embeds(r *rule.Rule, from label.Label) []label.Label {
	return []label.Label{}
}

// builtInNodeJSModules is a set of built-in Node.js modules.
//
// This set can be regenerated via the following command:
//
//     $ echo "require('module').builtinModules.forEach(m => console.log(m))" | nodejs
//
// See https://nodejs.org/api/module.html#module_module_builtinmodules.
var builtInNodeJSModules = map[string]bool{
	"_http_agent":         true,
	"_http_client":        true,
	"_http_common":        true,
	"_http_incoming":      true,
	"_http_outgoing":      true,
	"_http_server":        true,
	"_stream_duplex":      true,
	"_stream_passthrough": true,
	"_stream_readable":    true,
	"_stream_transform":   true,
	"_stream_wrap":        true,
	"_stream_writable":    true,
	"_tls_common":         true,
	"_tls_wrap":           true,
	"assert":              true,
	"async_hooks":         true,
	"buffer":              true,
	"child_process":       true,
	"cluster":             true,
	"console":             true,
	"constants":           true,
	"crypto":              true,
	"dgram":               true,
	"dns":                 true,
	"domain":              true,
	"events":              true,
	"fs":                  true,
	"http":                true,
	"http2":               true,
	"https":               true,
	"inspector":           true,
	"module":              true,
	"net":                 true,
	"os":                  true,
	"path":                true,
	"perf_hooks":          true,
	"process":             true,
	"punycode":            true,
	"querystring":         true,
	"readline":            true,
	"repl":                true,
	"stream":              true,
	"string_decoder":      true,
	"sys":                 true,
	"timers":              true,
	"tls":                 true,
	"trace_events":        true,
	"tty":                 true,
	"url":                 true,
	"util":                true,
	"v8":                  true,
	"vm":                  true,
	"wasi":                true,
	"worker_threads":      true,
	"zlib":                true,
}

// Resolve implements the resolve.Resolver interface.
func (fr *frontendResolver) Resolve(c *config.Config, _ *resolve.RuleIndex, _ *repo.RemoteCache, r *rule.Rule, imports interface{}, from label.Label) {
	importsFromRuleSources := imports.(importsParsedFromRuleSources)

	// fmt.Printf("**********\n")
	// fmt.Printf("Resolving rule %s\n", r.Name())
	// fmt.Printf("Deps: %v\n", r.AttrStrings("deps"))
	// fmt.Printf("imports: %v\n", importsFromRuleSources)
	// fmt.Printf("from: %v\n", from)

	switch {
	case r.Kind() == "karma_test" || r.Kind() == "nodejs_test" || r.Kind() == "sk_element_puppeteer_test" || r.Kind() == "ts_library":
		var deps []string
		for _, importPath := range importsFromRuleSources.getTypeScriptImports() {
			for _, ruleKindAndLabel := range fr.resolveDepsForTypeScriptImport(r.Kind(), from, importPath, c.RepoRoot) {
				// fmt.Printf("LABEL %s FROM RULE %s BECOMES %s\n", ruleKindAndLabel.label, from, ruleKindAndLabel.label.Rel(from.Repo, from.Pkg))
				deps = append(deps, ruleKindAndLabel.label.Rel(from.Repo, from.Pkg).String())
			}
		}
		setDeps(r, "deps", deps)

	case r.Kind() == "sass_library":
		var deps []string
		for _, importPath := range importsFromRuleSources.getSassImports() {
			ruleKindAndLabel := fr.resolveDepForSassImport(r.Kind(), from, importPath)
			if ruleKindAndLabel == noRuleKindAndLabel {
				continue // No rule satisfies the current Sass import. A warning has already been logged.
			}
			depLabel := ruleKindAndLabel.label
			if ruleKindAndLabel.kind == "sk_element" {
				// Ensure that the target name is explicit ("//my/package:package" vs "//my/package") before
				// appending the known suffix for the sass_library target generated by the sk_element macro.
				if depLabel.Name == "" {
					depLabel.Name = depLabel.Pkg
				}
				depLabel.Name = depLabel.Name + "_styles"
			}
			deps = append(deps, depLabel.Rel(from.Repo, from.Pkg).String())
		}
		setDeps(r, "deps", deps)

	case r.Kind() == "sk_element" || r.Kind() == "sk_page":
		var skElementDeps, tsDeps, sassDeps []string
		for _, importPath := range importsFromRuleSources.getTypeScriptImports() {
			for _, ruleKindAndLabel := range fr.resolveDepsForTypeScriptImport(r.Kind(), from, importPath, c.RepoRoot) {
				dep := ruleKindAndLabel.label.Rel(from.Repo, from.Pkg).String()
				if ruleKindAndLabel.kind == "sk_element" {
					skElementDeps = append(skElementDeps, dep)
				} else {
					tsDeps = append(tsDeps, dep)
				}
			}
		}
		for _, importPath := range importsFromRuleSources.getSassImports() {
			ruleKindAndLabel := fr.resolveDepForSassImport(r.Kind(), from, importPath)
			if ruleKindAndLabel == noRuleKindAndLabel {
				continue // No rule satisfies the current Sass import. A warning has already been logged.
			}
			dep := ruleKindAndLabel.label.Rel(from.Repo, from.Pkg).String()
			if ruleKindAndLabel.kind == "sk_element" {
				skElementDeps = append(skElementDeps, dep)
			} else {
				sassDeps = append(sassDeps, dep)
			}
		}
		setDeps(r, "sk_element_deps", skElementDeps)
		setDeps(r, "ts_deps", tsDeps)
		setDeps(r, "sass_deps", sassDeps)
	}
}

func setDeps(r *rule.Rule, depsAttr string, deps []string) {
	r.DelAttr(depsAttr)

	// Filter out self-imports (e.g. when an sk_element has files index.ts and foo-sk.ts, and
	// index.ts imports foo-sk.ts).
	deps = util.SSliceFilter(deps, func(s string) bool { return s != ":"+r.Name() })

	if len(deps) > 0 {
		deps = util.SSliceDedup(deps)
		sort.Strings(deps)
		r.SetAttr(depsAttr, deps)
		// fmt.Printf("RULE %s(name = %q): SETTING DEPS ATTR %s = %v\n", r.Kind(), r.Name(), depsAttr, deps)
	}
}

func (fr *frontendResolver) resolveDepForSassImport(ruleKind string, ruleLabel label.Label, importPath string) ruleKindAndLabel {
	// The elements-sk styles are a special case because they come from a genrule that copies them
	// from //infra-sk/node_modules/elements-sk into //bazel-bin/~elements-sk. These styles can be
	// accessed via the //infra-sk:elements-sk_scss sass_library.
	if strings.HasPrefix(importPath, "~elements-sk") {
		return ruleKindAndLabel{
			kind:  "sass_library",
			label: label.New("", "infra-sk", "elements-sk_scss"),
		}
	}

	// Sass always resolves imports relative to the current file first, so we normalize the import
	// path relative to the current directory, e.g. "../bar" imported from "myapp/foo" becomes
	// "myapp/bar".
	//
	// Reference:
	// https://sass-lang.com/documentation/at-rules/use#load-paths
	// https://sass-lang.com/documentation/at-rules/import#load-paths
	normalizedImportPath := path.Join(ruleLabel.Pkg, strings.TrimSuffix(importPath, path.Ext(importPath)))

	return fr.findRuleThatProvidesImport("sass", normalizedImportPath, ruleKind, ruleLabel)
}

func (fr *frontendResolver) resolveDepsForTypeScriptImport(ruleKind string, ruleLabel label.Label, importPath string, repoRootDir string) []ruleKindAndLabel {
	// Is this an import of another source file in the repository?
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		// Normalize the import path, e.g. "../bar" imported from "myapp/foo" becomes "myapp/bar".
		normalizedImportPath := path.Join(ruleLabel.Pkg, importPath)

		rkal := fr.findRuleThatProvidesImport("ts", normalizedImportPath, ruleKind, ruleLabel)
		if rkal == noRuleKindAndLabel {
			return []ruleKindAndLabel{}
		}
		return []ruleKindAndLabel{rkal}
	}

	// The import must be either an NPM package or a built-in Node.js module.
	moduleName := strings.Split(importPath, "/")[0] // e.g. my-module/foo/bar => my-module

	// Is this an import from an NPM package?
	if npmPackages := fr.getNPMPackages(filepath.Join(repoRootDir, "infra-sk", "package.json")); npmPackages[moduleName] {
		var rkals []ruleKindAndLabel
		// Add as dependencies both the module and its type annotations package, if it exists.
		rkals = append(rkals, ruleKindAndLabel{
			kind:  "",                                                   // This dependency is not a rule (e.g. ts_library), so we leave the rule kind blank.
			label: label.New(npmBazelNamespace, moduleName, moduleName), // e.g. @infra-sk_npm//puppeteer
		})
		typesModuleName := "@types/" + moduleName // e.g. @types/my-module
		if npmPackages[typesModuleName] {
			rkals = append(rkals, ruleKindAndLabel{
				kind:  "",                                                        // This dependency is not a rule (e.g. ts_library), so we leave the rule kind blank.
				label: label.New(npmBazelNamespace, typesModuleName, moduleName), // e.g. @infra-sk_npm//@types/puppeteer
			})
		}
		return rkals
	}

	// Is this a built-in Node.js module?
	if builtInNodeJSModules[moduleName] {
		// Nothing to do - no need to add built-in modules as explicit dependencies.
		return []ruleKindAndLabel{}
	}

	log.Printf("Unable to resolve import %q from %s (%s): no %q NPM package or built-in module found.", importPath, ruleLabel, ruleKind, moduleName)
	return []ruleKindAndLabel{}
}

// getNPMPackages returns the set of NPM dependencies found in the package.json file.
func (fr *frontendResolver) getNPMPackages(path string) map[string]bool {
	if fr.npmPackages != nil {
		return fr.npmPackages
	}

	var packageJSON struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	// Read in and unmarshall package.json file.
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("Error reading file %q: %v", path, err)
	}
	if err := json.Unmarshal(b, &packageJSON); err != nil {
		log.Panicf("Error parsing %s: %v", path, err)
	}

	// Extract all NPM packages found in the package.json file.
	fr.npmPackages = map[string]bool{}
	for pkg := range packageJSON.Dependencies {
		fr.npmPackages[pkg] = true
	}
	for pkg := range packageJSON.DevDependencies {
		fr.npmPackages[pkg] = true
	}

	return fr.npmPackages
}

var _ resolve.Resolver = &frontendResolver{}

//////////////
// Language //
//////////////

// frontendLang implements the language.Language interface.
type frontendLang struct {
	frontendConfigurer
	frontendResolver
}

// Kinds implements the language.Language interface.
func (fl *frontendLang) Kinds() map[string]rule.KindInfo {
	return map[string]rule.KindInfo{
		"karma_test": {
			NonEmptyAttrs:  map[string]bool{"src": true},
			MergeableAttrs: map[string]bool{"src": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		},
		"nodejs_test": {
			NonEmptyAttrs:  map[string]bool{"src": true},
			MergeableAttrs: map[string]bool{"src": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		},
		"sass_library": {
			NonEmptyAttrs:  map[string]bool{"srcs": true},
			MergeableAttrs: map[string]bool{"srcs": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		},
		"sk_demo_page_server": {
			NonEmptyAttrs:  map[string]bool{"sk_page": true},
			MergeableAttrs: map[string]bool{"sk_page": true},
		},
		"sk_element": {
			MatchAny:       true,
			NonEmptyAttrs:  map[string]bool{"ts_srcs": true, "sass_srcs": true},
			MergeableAttrs: map[string]bool{"ts_srcs": true, "sass_srcs": true},
			ResolveAttrs:   map[string]bool{"sass_deps": true, "sk_element_deps": true, "ts_deps": true},
		},
		"sk_element_puppeteer_test": {
			NonEmptyAttrs:  map[string]bool{"src": true, "sk_demo_page_server": true},
			MergeableAttrs: map[string]bool{"src": true, "sk_demo_page_server": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		},
		"sk_page": {
			NonEmptyAttrs:  map[string]bool{"html_file": true, "ts_entry_point": true, "scss_entry_point": true},
			MergeableAttrs: map[string]bool{"html_file": true, "ts_entry_point": true, "scss_entry_point": true},
			ResolveAttrs:   map[string]bool{"sass_deps": true, "sk_element_deps": true, "ts_deps": true},
		},
		"ts_library": {
			NonEmptyAttrs:  map[string]bool{"srcs": true},
			MergeableAttrs: map[string]bool{"srcs": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		},
	}
}

// Loads implements the language.Language interface.
func (fl *frontendLang) Loads() []rule.LoadInfo {
	return []rule.LoadInfo{
		{
			Name:    "//infra-sk:index.bzl",
			Symbols: []string{"karma_test", "nodejs_test", "sass_library", "sk_demo_page_server", "sk_element", "sk_element_puppeteer_test", "sk_page", "ts_library"},
		},
	}
}

type importsParsedFromRuleSources interface {
	getSassImports() []string
	getTypeScriptImports() []string
}

type importsParsedFromRuleSourcesImpl struct {
	sassImports []string
	tsImports   []string
}

func (i *importsParsedFromRuleSourcesImpl) getSassImports() []string {
	return i.sassImports
}

func (i *importsParsedFromRuleSourcesImpl) getTypeScriptImports() []string {
	return i.tsImports
}

var _ importsParsedFromRuleSources = &importsParsedFromRuleSourcesImpl{}

// GenerateRules implements the language.Language interface.
func (fl *frontendLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	// Skip known directories with third-party code.
	for _, dir := range strings.Split(args.Rel, "/") {
		if util.In(dir, []string{"node_modules", "bower_components"}) {
			return language.GenerateResult{}
		}
	}

	// Limit generation of build targets to a hard-coded list of known good directories.
	// TODO(lovisolo): Delete.
	if !isTargetDirectory(args.Rel) {
		return language.GenerateResult{}
	}

	// fmt.Printf("GENERATE RULES: %s, files: %v\n", args.Rel, append(args.RegularFiles, args.GenFiles...))

	var rules []*rule.Rule
	var imports []importsParsedFromRuleSources

	allFiles := append(args.RegularFiles, args.GenFiles...)

	// Application page directories follow the "<app>/pages" pattern.
	if isAppPageDir(args.Dir) {
		// This map will store the source files of any application pages found in the current directory.
		pages := map[string]*skPageSrcs{}
		getPage := func(name string) *skPageSrcs {
			if pages[name] == nil {
				pages[name] = &skPageSrcs{}
			}
			return pages[name]
		}

		// Populate the pages map with all HTML, TypeScript and Sass files found in the directory.
		for _, f := range allFiles {
			if strings.HasSuffix(f, "_test.ts") {
				log.Printf("Ignoring TypeScript test file found in directory with application pages: %s", filepath.Join(args.Dir, f))
				continue
			}

			name := strings.TrimSuffix(f, filepath.Ext(f)) // e.g. my-page.html -> my-page
			switch filepath.Ext(f) {
			case ".html":
				getPage(name).html = f
			case ".ts":
				getPage(name).ts = f
			case ".scss":
				getPage(name).scss = f
			}
		}

		// Generate sk_page targets for all pages for which we have all the necessary files (i.e.
		// my-page.html and my-page.ts), or stand-alone ts_library and sass_library targets for any
		// TypeScript and Sass files that do not belong to a page.
		for _, page := range pages {
			// A page is valid if it has an HTML file and a TypeScript file.
			if page.isValid() {
				r, i := generateSkPageRule(page, args.Dir)
				rules = append(rules, r)
				imports = append(imports, i)
			} else {
				if page.ts != "" {
					r, i := generateTSLibraryRule(page.ts, args.Dir)
					rules = append(rules, r)
					imports = append(imports, i)
				}
				if page.scss != "" {
					r, i := generateSassLibraryRule(page.scss, args.Dir)
					rules = append(rules, r)
					imports = append(imports, i)
				}
			}
		}

		return makeGenerateResult(args, rules, imports)
	}

	// Custom element directories follow the "<app>/modules/<element-name-sk>" pattern.
	isCustomElementDir, customElementName := extractCustomElementNameFromDir(args.Dir)

	// If we are in a custom element directory, it will contain at most one custom element and one
	// custom page. Let's find the source files for both, and generate the corresponding sk_element,
	// sk_page and sk_demo_page_server rules.
	customElementSrcs := &skElementSrcs{}
	demoPageSrcs := &skPageSrcs{}
	var skDemoPageServerRule *rule.Rule // We'll need it later for the Puppeteer test, if there's one.
	if isCustomElementDir {
		// Iterate over all files and add them to the appropriate structs.
		indexTsFound := false
		for _, f := range allFiles {
			switch f {
			case "index.ts":
				indexTsFound = true
			case customElementName + ".ts":
				customElementSrcs.ts = f
			case customElementName + ".scss":
				customElementSrcs.scss = f
			case customElementName + "-demo.html":
				demoPageSrcs.html = f
			case customElementName + "-demo.ts":
				demoPageSrcs.ts = f
			case customElementName + "-demo.scss":
				demoPageSrcs.scss = f
			}
		}

		// An index.ts file alone does not make an sk_element, so we will include it in the returned
		// skElementSrcs struct only if the struct has other sources as well.
		if indexTsFound && !customElementSrcs.isEmpty() {
			customElementSrcs.indexTs = "index.ts"
		}

		// Generate the rules.
		if customElementSrcs.isValid() {
			r, i := generateSkElementRule(customElementName, customElementSrcs, args.Dir)
			rules = append(rules, r)
			imports = append(imports, i)
		}
		if demoPageSrcs.isValid() {
			r, i := generateSkPageRule(demoPageSrcs, args.Dir)
			rules = append(rules, r)
			imports = append(imports, i)

			skDemoPageServerRule, i = generateSkDemoPageServerRule(r.Name())
			rules = append(rules, skDemoPageServerRule)
			imports = append(imports, i)
		}
	}

	// Generate rules for all other files found in the directory.
	for _, f := range allFiles {
		if isCustomElementDir {
			// Skip any files that belong to the custom element or demo page.
			if (customElementSrcs.isValid() && customElementSrcs.has(f)) || (demoPageSrcs.isValid() && demoPageSrcs.has(f)) {
				continue
			}
		}

		if strings.HasSuffix(f, ".scss") {
			r, i := generateSassLibraryRule(f, args.Dir)
			rules = append(rules, r)
			imports = append(imports, i)
		} else if strings.HasSuffix(f, "_nodejs_test.ts") {
			r, i := generateNodeJSTestRule(f, args.Dir)
			rules = append(rules, r)
			imports = append(imports, i)
		} else if strings.HasSuffix(f, "_puppeteer_test.ts") && skDemoPageServerRule != nil {
			r, i := generateSkElementPuppeteerTestRule(f, args.Dir, skDemoPageServerRule.Name())
			rules = append(rules, r)
			imports = append(imports, i)
		} else if strings.HasSuffix(f, "_test.ts") {
			r, i := generateKarmaTestRule(f, args.Dir)
			rules = append(rules, r)
			imports = append(imports, i)
		} else if strings.HasSuffix(f, ".ts") {
			r, i := generateTSLibraryRule(f, args.Dir)
			rules = append(rules, r)
			imports = append(imports, i)
		}
	}

	return makeGenerateResult(args, rules, imports)
}

func makeGenerateResult(args language.GenerateArgs, rules []*rule.Rule, imports []importsParsedFromRuleSources) language.GenerateResult {
	// The Imports field in language.GenerateResult is of type []interface{}, so we need to cast our
	// imports slice to []interface{}.
	var importsAsEmptyInterfaces []interface{}
	for _, i := range imports {
		importsAsEmptyInterfaces = append(importsAsEmptyInterfaces, i.(interface{}))
	}

	return language.GenerateResult{
		Gen:     rules,
		Imports: importsAsEmptyInterfaces,
		Empty:   generateEmptyRules(args),
	}
}

var (
	// appPagesDirRegexp matches directories where application pages are found (sk_page targets), e.g.
	// "myapp/pages".
	appPagesDirRegexp = regexp.MustCompile(`(?P<app_name>(?:[[:alnum:]]|_|-)+)/pages`)

	// skElementModuleDirRegexp matches directories that might contain an sk_element, e.g.
	// "myapp/modules/my-element-sk".
	skElementModuleDirRegexp = regexp.MustCompile(`(?P<app_name>(?:[[:alnum:]]|_|-)+)/modules/(?P<element_name>(?:[[:alnum:]]|_|-)+-sk)`)
)

// isAppPageDir returns true if the directory matches the "<app>/pages" pattern, which indicates it
// might contain application pages (sk_page targets).
func isAppPageDir(dir string) bool {
	return appPagesDirRegexp.MatchString(dir)
}

// extractCustomElementNameFromDir determines whether the given directory corresponds to a custom
// element based on the "<app>/modules/<element-name-sk>" pattern, and returns the element name if
// the directory matches said pattern.
func extractCustomElementNameFromDir(dir string) (bool, string) {
	match := skElementModuleDirRegexp.FindStringSubmatch(dir)
	if len(match) != 3 {
		return false, ""
	}
	return true, match[2]
}

// skElementSrcs groups together the various sources that could make an sk_element target.
type skElementSrcs struct {
	indexTs string // index.ts
	ts      string // my-element-sk.ts
	scss    string // my-element-sk.scss
}

// isValid returns true if the struct contains the necessary sources to build an sk_element, or
// false otherwise.
func (e *skElementSrcs) isValid() bool {
	return e.ts != ""
}

// isEmpty returns true if the structure does not contain any source files.
func (e *skElementSrcs) isEmpty() bool {
	return *e == skElementSrcs{}
}

// has returns true if the struct includes the given source file, or false otherwise.
func (e *skElementSrcs) has(src string) bool {
	return src == e.indexTs || src == e.ts || src == e.scss
}

// skPageSrcs groups together the various sources that could make an sk_page target.
type skPageSrcs struct {
	html string // my-element-sk-demo.html
	ts   string // my-element-sk-demo.ts
	scss string // my-element-sk-demo.scss
}

// isValid returns true if the struct contains the necessary sources to build an sk_page, or false
// otherwise.
func (p *skPageSrcs) isValid() bool {
	return p.html != "" && p.ts != ""
}

// has returns true if the struct includes the given source file, or false otherwise.
func (p *skPageSrcs) has(src string) bool {
	return src == p.html || src == p.ts || src == p.scss
}

// generateSkElementRule generates a sk_element rule for the given sources.
func generateSkElementRule(name string, srcs *skElementSrcs, dir string) (*rule.Rule, importsParsedFromRuleSources) {
	rule := rule.NewRule("sk_element", name)
	if srcs.indexTs != "" {
		rule.SetAttr("ts_srcs", []string{srcs.indexTs, srcs.ts})
	} else {
		rule.SetAttr("ts_srcs", []string{srcs.ts})
	}
	rule.SetAttr("sass_srcs", []string{srcs.scss})
	rule.SetAttr("visibility", []string{"//visibility:public"})

	imports := &importsParsedFromRuleSourcesImpl{}
	if srcs.indexTs != "" {
		imports.tsImports = append(imports.tsImports, extractImportsFromTypeScriptFile(filepath.Join(dir, srcs.indexTs))...)
	}
	imports.tsImports = append(imports.tsImports, extractImportsFromTypeScriptFile(filepath.Join(dir, srcs.ts))...)
	if srcs.scss != "" {
		imports.sassImports = append(imports.sassImports, extractImportsFromSassFile(filepath.Join(dir, srcs.scss))...)
	}

	return rule, imports
}

// generateSkPageRule generates a sk_page rule for the given sources.
func generateSkPageRule(srcs *skPageSrcs, dir string) (*rule.Rule, importsParsedFromRuleSources) {
	rule := rule.NewRule("sk_page", makeRuleNameFromFileName(srcs.html))
	rule.SetAttr("html_file", srcs.html)
	rule.SetAttr("ts_entry_point", srcs.ts)
	if srcs.scss != "" {
		rule.SetAttr("scss_entry_point", srcs.scss)
	}

	imports := &importsParsedFromRuleSourcesImpl{
		tsImports: extractImportsFromTypeScriptFile(filepath.Join(dir, srcs.ts)),
	}
	if srcs.scss != "" {
		imports.sassImports = extractImportsFromSassFile(filepath.Join(dir, srcs.scss))
	}

	return rule, imports
}

// generateSkDemoPageServerRule generates a sk_demo_page_server rule for the given sk_page.
func generateSkDemoPageServerRule(skPage string) (*rule.Rule, importsParsedFromRuleSources) {
	rule := rule.NewRule("sk_demo_page_server", "demo_page_server")
	rule.SetAttr("sk_page", skPage)
	return rule, &importsParsedFromRuleSourcesImpl{}
}

// generateSassLibraryRule generates a sass_library rule for the given Sass file.
func generateSassLibraryRule(file, dir string) (*rule.Rule, importsParsedFromRuleSources) {
	rule := rule.NewRule("sass_library", makeRuleNameFromFileName(file))
	rule.SetAttr("srcs", []string{file})
	rule.SetAttr("visibility", []string{"//visibility:public"})
	return rule, &importsParsedFromRuleSourcesImpl{sassImports: extractImportsFromSassFile(filepath.Join(dir, file))}
}

// generateKarmaTestRule generates a karma_test rule for the given TypeScript file.
func generateKarmaTestRule(file, dir string) (*rule.Rule, importsParsedFromRuleSources) {
	rule := rule.NewRule("karma_test", makeRuleNameFromFileName(file))
	rule.SetAttr("src", file)
	return rule, &importsParsedFromRuleSourcesImpl{tsImports: extractImportsFromTypeScriptFile(filepath.Join(dir, file))}
}

// generateNodeJSTestRule generates a nodejs_test rule for the given TypeScript file.
func generateNodeJSTestRule(file, dir string) (*rule.Rule, importsParsedFromRuleSources) {
	rule := rule.NewRule("nodejs_test", makeRuleNameFromFileName(file))
	rule.SetAttr("src", file)
	return rule, &importsParsedFromRuleSourcesImpl{tsImports: extractImportsFromTypeScriptFile(filepath.Join(dir, file))}
}

// generateSkElementPuppeteerTestRule generates a sk_element_puppeteer_test rule for the given
// TypeScript file and sk_demo_page_server.
func generateSkElementPuppeteerTestRule(file, dir, skDemoPageServer string) (*rule.Rule, importsParsedFromRuleSources) {
	rule := rule.NewRule("sk_element_puppeteer_test", makeRuleNameFromFileName(file))
	rule.SetAttr("src", file)
	rule.SetAttr("sk_demo_page_server", skDemoPageServer)
	return rule, &importsParsedFromRuleSourcesImpl{tsImports: extractImportsFromTypeScriptFile(filepath.Join(dir, file))}
}

// generateTSLibraryRule generates a ts_library rule for the given TypeScript file.
func generateTSLibraryRule(file, dir string) (*rule.Rule, importsParsedFromRuleSources) {
	rule := rule.NewRule("ts_library", makeRuleNameFromFileName(file))
	rule.SetAttr("srcs", []string{file})
	rule.SetAttr("visibility", []string{"//visibility:public"})
	return rule, &importsParsedFromRuleSourcesImpl{tsImports: extractImportsFromTypeScriptFile(filepath.Join(dir, file))}
}

// makeRuleNameFromFileName returns e.g. "baz_test" when given "foo/bar/baz_test.ts".
func makeRuleNameFromFileName(file string) string {
	file = strings.ToLower(path.Base(file))
	return strings.TrimSuffix(file, filepath.Ext(file))
}

// extractImportsFromSassFile returns the verbatim paths of the import statements found in the given
// Sass file.
func extractImportsFromSassFile(path string) []string {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("Error reading file %q: %v", path, err)
	}
	return parseSassImports(string(b[:]))
}

// extractImportsFromTypeScriptFile returns the verbatim paths of the import statements found in the
// given TypeScript file.
func extractImportsFromTypeScriptFile(path string) []string {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("Error reading file %q: %v", path, err)
	}

	// Ignore CSS / Sass imports from TypeScript files (Webpack idiom).
	return util.SSliceFilter(parseTSImports(string(b[:])), func(s string) bool {
		return !strings.HasSuffix(s, ".css") && !strings.HasSuffix(s, ".scss")
	})
}

// generateEmptyRules returns a list of rules that cannot be built with the files found in the
// directory, for example because a file in its srcs argument does not exist anymore.
//
// Gazelle will merge these rules with the existing rules, and if any of their attributes marked as
// non-empty are empty after the merge, they will be deleted.
func generateEmptyRules(args language.GenerateArgs) []*rule.Rule {
	var emptyRules []*rule.Rule

	// If no BUILD.bazel file exists in the current directory, there's nothing to do.
	if args.File == nil {
		return emptyRules
	}

	allFilesInDir := map[string]bool{}
	for _, f := range append(args.RegularFiles, args.GenFiles...) {
		allFilesInDir[f] = true
	}

	someFilesFound := func(files ...string) bool {
		for _, f := range files {
			if allFilesInDir[f] {
				return true
			}
		}
		return false
	}

	allFilesFound := func(files ...string) bool {
		for _, f := range files {
			if !allFilesInDir[f] {
				return false
			}
		}
		return true
	}

	allRulesByNameInDir := map[string]*rule.Rule{}
	for _, r := range args.File.Rules {
		allRulesByNameInDir[r.Name()] = r
	}

	ruleFound := func(kind, name string) bool {
		r := allRulesByNameInDir[name]
		return r != nil && r.Kind() == kind
	}

	for _, curRule := range args.File.Rules {
		var empty bool

		switch curRule.Kind() {
		case "karma_test":
			empty = !someFilesFound(curRule.AttrString("src"))
		case "nodejs_test":
			empty = !someFilesFound(curRule.AttrString("src"))
		case "sass_library":
			empty = !someFilesFound(curRule.AttrStrings("srcs")...)
		case "sk_demo_page_server":
			empty = !ruleFound("sk_page", curRule.AttrString("sk_page"))
		case "sk_element":
			empty = !someFilesFound(curRule.AttrStrings("ts_srcs")...)
		case "sk_element_puppeteer_test":
			empty = !ruleFound("sk_demo_page_server", curRule.AttrString("sk_demo_page_server"))
		case "sk_page":
			empty = !allFilesFound(curRule.AttrString("html_file"), curRule.AttrString("ts_entry_point"))
		case "ts_library":
			empty = !someFilesFound(curRule.AttrStrings("srcs")...)
		}

		if empty {
			emptyRules = append(emptyRules, rule.NewRule(curRule.Kind(), curRule.Name()))
		}
	}

	return emptyRules
}

// Fix implements the language.Language interface.
func (fl *frontendLang) Fix(c *config.Config, f *rule.File) {
}

var _ language.Language = &frontendLang{}

// NewLanguage returns an instance of the Gazelle extension for Skia Infrastructure front-end code.
//
// This function is called from the Gazelle binary.
func NewLanguage() language.Language {
	return &frontendLang{}
}
