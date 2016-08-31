// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package resp

// Settings denotes a full renderable Milo settings page.
type Settings struct {
	// Where the form should go.
	ActionURL string

	// Themes is a list of usable themes for Milo
	Theme *Choices
}

// Choices - A dropdown menu showing all possible choices.
type Choices struct {
	// A list of all possible choices.
	Choices []string

	// The selected choice.
	Selected string
}
