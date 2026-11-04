// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/oauth2redirect"
)

func main() {
	sklog.Fatal(oauth2redirect.Main())
}
