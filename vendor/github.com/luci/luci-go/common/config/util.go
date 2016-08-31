// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"fmt"
)

// ProjectConfigSet returns the name of the ConfigSet associated with project.
func ProjectConfigSet(project ProjectName) string {
	return fmt.Sprintf("projects/%s", project)
}
