package frontend

import (
	gazelle "github.com/bazelbuild/bazel-gazelle/language"
	"go.skia.org/infra/bazel/gazelle/frontend/language"
)

// NewLanguage returns an instance of the Gazelle extension for Skia Infrastructure front-end code.
//
// This function is called from the Gazelle binary.
func NewLanguage() gazelle.Language {
	return &language.Language{
		TargetDirectories: targetDirectories,
	}
}
