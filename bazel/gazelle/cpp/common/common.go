// Package common contains any code used by two or more packages. It avoid circular dependencies.
package common

import (
	"strings"
)

const GeneratedCCAtomRule = "generated_cc_atom"

// ImportsParsedFromRuleSources is the "imports" interface returned by Language.GenerateRules(), and
// passed by Gazelle to Resolver.Resolve().
type ImportsParsedFromRuleSources interface {
	// GetRepoHeaders returns the files that are presumed to be in the repo. This means they are
	// source files in this project, or deps in third_party. These files should be resolved and
	// added to the dependency list for this rule.
	GetRepoIncludes() []string
	// GetSystemHeaders returns the files that are presumed to be provided via the toolchain.
	// Therefore, they do not need to be added to dependencies. As such, they are currently ignored,
	// but could be useful in the future.
	GetSystemIncludes() []string
	// GetThirdPartyMap returns a mapping from file name to a Bazel label string that is used to
	// resolve files returned from GetRepoHeaders() if they match.
	GetThirdPartyMap() map[string]string
}

// importsParsedFromRuleSourcesImpl implements the common.ImportsParsedFromRuleSources interface.
type importsParsedFromRuleSourcesImpl struct {
	repoHeaders   []string
	systemHeaders []string

	thirdPartyMap map[string]string
}

func (i importsParsedFromRuleSourcesImpl) GetSystemIncludes() []string {
	return i.systemHeaders
}

func (i importsParsedFromRuleSourcesImpl) GetRepoIncludes() []string {
	return i.repoHeaders
}

func (i importsParsedFromRuleSourcesImpl) GetThirdPartyMap() map[string]string {
	return i.thirdPartyMap
}

func NewImports(repoHeaders, systemHeaders []string, thirdPartyMap map[string]string) ImportsParsedFromRuleSources {
	return importsParsedFromRuleSourcesImpl{
		repoHeaders:   repoHeaders,
		systemHeaders: systemHeaders,
		thirdPartyMap: thirdPartyMap,
	}
}

var _ ImportsParsedFromRuleSources = importsParsedFromRuleSourcesImpl{}

// TrimCSuffix removes the .h, .cpp, etc suffix from the given file.
func TrimCSuffix(name string) string {
	return strings.TrimSuffix(name, getSuffix(name))
}

// From https://docs.bazel.build/versions/main/be/c-cpp.html#cc_binary.srcs
var cppHeaders = []string{".h", ".hh", ".hpp", ".hxx", ".inc", ".in", ".H"}
var cppSrcs = []string{".c", ".cc", ".cpp", ".cxx", ".c++", ".C"}

// IsCppHeader returns true if the given file name ends with one of the suffixes that Bazel
// recognizes as a C or C++ header file.
func IsCppHeader(name string) bool {
	suffix := getSuffix(name)
	for _, ext := range cppHeaders {
		if suffix == ext {
			return true
		}
	}
	return false
}

// IsCppSource returns true if the given file name ends with one of the suffixes that Bazel
// recognizes as a C or C++ source file.
func IsCppSource(name string) bool {
	suffix := getSuffix(name)
	for _, ext := range cppSrcs {
		if suffix == ext {
			return true
		}
	}
	return false
}

func getSuffix(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}
