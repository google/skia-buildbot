// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the genpromcrd command line application.
package main

import (
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/promk/go/genpromcrd/genpromcrd"
)

func main() {
	app := genpromcrd.NewApp()

	if err := app.Main(os.Args); err != nil {
		if err != genpromcrd.ErrFlagsParse {
			sklog.Fatal(err)
		}
	}
}
