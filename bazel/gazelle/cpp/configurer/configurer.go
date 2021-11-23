package configurer

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"go.skia.org/infra/bazel/gazelle/cpp/common"
	"go.skia.org/infra/go/skerr"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// CppConfigurer implements the config.Configurer interface.
type CppConfigurer struct {
	thirdPartyFile    string
	ThirdPartyFileMap map[string]string
}

// RegisterFlags adds any flags this extension takes.
func (c *CppConfigurer) RegisterFlags(fs *flag.FlagSet, _ string, _ *config.Config) {
	fs.StringVar(&c.thirdPartyFile, "third_party_file_map", "", "This file should be a JSON dictionary that maps include paths to third_party Bazel labels.")
}

// CheckFlags processes the flags defined by this extension. Concretely, it attempts to read
// in the passed-in third_party_file_map, if one was specified.
func (c *CppConfigurer) CheckFlags(*flag.FlagSet, *config.Config) error {
	if c.thirdPartyFile == "" {
		log.Printf("No third_party_file_map configured")
		c.ThirdPartyFileMap = map[string]string{}
		return nil
	}
	b, err := os.ReadFile(c.thirdPartyFile)
	if err != nil {
		return skerr.Wrapf(err, "Reading third_party_file_map %s", c.thirdPartyFile)
	}
	if err := json.Unmarshal(b, &c.ThirdPartyFileMap); err != nil {
		return skerr.Wrapf(err, "Parsing JSON in %s", c.thirdPartyFile)
	}
	return nil
}

// KnownDirectives implements the config.Configurer interface.
//
// Interface documentation:
//
// KnownDirectives returns a list of directive keys that this CppConfigurer can
// interpret. Gazelle prints errors for directives that are not recognized by
// any CppConfigurer.
func (c *CppConfigurer) KnownDirectives() []string {
	return []string{common.GeneratedCCAtomRule}
}

// Configure implements the config.Configurer interface.
func (c *CppConfigurer) Configure(*config.Config, string, *rule.File) {}

var _ config.Configurer = &CppConfigurer{}
