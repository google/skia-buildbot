// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package poolstest

import "go.skia.org/infra/machine/go/machineserver/config"

var PoolConfigForTesting = config.InstanceConfig{
	Pools: []config.Pool{
		{
			Name:  "SkiaInternal",
			Regex: "^skia-i-",
		},
		{
			Name:  "Skia",
			Regex: "^skia-",
		},
	},
}
