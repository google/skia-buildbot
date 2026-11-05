// Copyright 2024 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Program to generate JSON Schema definitions for the Tool struct.
//
//go:generate bazelisk run --config=mayberemote //:go -- run .
package main

import (
	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/tool/go/tool"
)

func main() {
	jsonschema.GenerateSchema("../schema.json", &tool.Tool{})
}
