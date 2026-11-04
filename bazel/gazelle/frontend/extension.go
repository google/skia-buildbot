// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontend

import (
	gazelle "github.com/bazelbuild/bazel-gazelle/language"
	"go.skia.org/infra/bazel/gazelle/frontend/language"
)

// NewLanguage returns an instance of the Gazelle extension for Skia Infrastructure front-end code.
//
// This function is called from the Gazelle binary.
func NewLanguage() gazelle.Language {
	return &language.Language{}
}
