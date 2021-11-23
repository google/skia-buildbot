package cpp

import (
	gazelle "github.com/bazelbuild/bazel-gazelle/language"
	"go.skia.org/infra/bazel/gazelle/cpp/language"
)

// NewLanguage returns an instance of the Gazelle extension that can generate C++ rules.
//
// This function is called from the Gazelle binary, but may appear unused by some IDEs.
func NewLanguage() gazelle.Language {
	return &language.CppLanguage{}
}
