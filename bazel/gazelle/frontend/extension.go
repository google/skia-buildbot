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
// generates ts_library, karma_test and sass_library targets for those files, respectively.
//
// A Gazelle extension is essentially a go_library with a function named NewLanguage that provides
// an implementation of the language.Language interface. This interface provides hooks for
// generating rules, parsing configuration directives, and resolving imports to Bazel labels.
//
// Docs on Gazelle extensions: https://github.com/bazelbuild/bazel-gazelle/blob/master/extend.rst.
package frontend

import (
	"flag"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

////////////////
// Configurer //
////////////////

// frontendConfigurer implements the config.Configurer interface.
type frontendConfigurer struct{}

// RegisterFlags implements the config.Configurer interface.
func (fc *frontendConfigurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
}

// CheckFlags implements the config.Configurer interface.
func (fc *frontendConfigurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}

// KnownDirectives implements the config.Configurer interface.
func (fc *frontendConfigurer) KnownDirectives() []string {
	return []string{}
}

// Configure implements the config.Configurer interface.
func (fc *frontendConfigurer) Configure(c *config.Config, rel string, f *rule.File) {
}

var _ config.Configurer = &frontendConfigurer{}

//////////////
// Resolver //
//////////////

// frontendResolver implements the resolve.Resolver interface.
type frontendResolver struct{}

// Name implements the resolve.Resolver interface.
func (fr *frontendResolver) Name() string {
	return ""
}

// Imports implements the resolve.Resolver interface.
func (fr *frontendResolver) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	return []resolve.ImportSpec{}
}

// Embeds implements the resolve.Resolver interface.
func (fr *frontendResolver) Embeds(r *rule.Rule, from label.Label) []label.Label {
	return []label.Label{}
}

// Resolve implements the resolve.Resolver interface.
func (fr *frontendResolver) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports interface{}, from label.Label) {
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
	return map[string]rule.KindInfo{}
}

// Loads implements the language.Language interface.
func (fl *frontendLang) Loads() []rule.LoadInfo {
	return []rule.LoadInfo{}
}

// GenerateRules implements the language.Language interface.
func (fl *frontendLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	return language.GenerateResult{}
}

// Fix implements the language.Language interface.
func (fl *frontendLang) Fix(c *config.Config, f *rule.File) {
}

var _ language.Language = &frontendLang{}

// NewLanguage returns an instance of the Gazelle extension for Skia Infrastructure front-end code.
func NewLanguage() language.Language {
	return &frontendLang{}
}