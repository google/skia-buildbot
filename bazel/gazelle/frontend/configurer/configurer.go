package configurer

import (
	"flag"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// Configurer implements the config.Configurer interface.
type Configurer struct{}

// RegisterFlags implements the config.Configurer interface.
func (c *Configurer) RegisterFlags(fs *flag.FlagSet, cmd string, cc *config.Config) {}

// CheckFlags implements the config.Configurer interface.
func (c *Configurer) CheckFlags(fs *flag.FlagSet, cc *config.Config) error { return nil }

// KnownDirectives implements the config.Configurer interface.
//
// Interface documentation:
//
// KnownDirectives returns a list of directive keys that this Configurer can
// interpret. Gazelle prints errors for directives that are not recoginized by
// any Configurer.
func (c *Configurer) KnownDirectives() []string {
	return []string{"karma_test", "nodejs_test", "sass_library", "sk_demo_page_server", "sk_element", "sk_element_puppeteer_test", "sk_page", "ts_library"}
}

// Configure implements the config.Configurer interface.
func (c *Configurer) Configure(cc *config.Config, rel string, f *rule.File) {}

var _ config.Configurer = &Configurer{}
