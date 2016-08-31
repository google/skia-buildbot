// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/server/templates"
)

// TestableSettings is a subclass of Settings that interfaces with
// TestableHandler and includes sample test data.
type TestableSettings struct{ Settings }

// TestData returns sample test data.
func (s TestableSettings) TestData() []TestBundle {
	return []TestBundle{
		{
			Description: "Settings",
			Data: templates.Args{
				"Settings": &resp.Settings{
					ActionURL: "/post/action",
					Theme: &resp.Choices{
						Choices:  GetAllThemes(),
						Selected: "bootstrap",
					},
				},
				"XsrfToken": "thisisatoken",
			},
		},
	}
}
