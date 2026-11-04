// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy"
)

func main() {
	if err := authproxy.Main(); err != nil {
		sklog.Fatal(err)
	}
}
