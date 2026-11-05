// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Program to generate JSON Schema definitions for the Probe struct.
//
//go:generate bazelisk run --config=mayberemote //:go -- run .
package main

import (
	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/proberk/go/types"
)

func main() {
	jsonschema.GenerateSchema("../probesSchema.json", &types.Probes{})
}
