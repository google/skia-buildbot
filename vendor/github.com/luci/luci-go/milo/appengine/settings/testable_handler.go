// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Data Structures used for defining test data for use with testing each theme.

package settings

import (
	"github.com/luci/luci-go/server/templates"
)

// TestableHandler pairs up a handler with a list of test data.
type TestableHandler interface {
	// ThemedHandler represents the Hanler that this TestableHandler is associated with.
	ThemedHandler
	// TestData returns a list of example data used for testing if a theme renders correctly.
	TestData() []TestBundle
}

// TestBundle is a template arg associated with a description used for testing.
type TestBundle struct {
	// Description is a short one line description of what the data contains.
	Description string
	// Data is the data fed directly into the template.
	Data templates.Args
}
