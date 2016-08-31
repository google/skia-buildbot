// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/logdog/common/types"
)

func makeUnifiedPath(project config.ProjectName, path types.StreamPath) string {
	val := string(project)
	if path != "" {
		val += types.StreamNameSepStr + string(path)
	}
	return val
}
