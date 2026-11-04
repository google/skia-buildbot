// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package config

import (
	_ "embed"
)

//go:embed config.json
var NpmAuditMirrorConfig string

//go:embed verdaccio-config.tmpl
var VerdaccioConfigTemplate string
