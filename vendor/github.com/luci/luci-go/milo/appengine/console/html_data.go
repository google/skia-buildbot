// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package console

import "github.com/luci/luci-go/milo/appengine/settings"

// TestableConsole is a subclass of Build that interfaces with TestableHandler and
// includes sample test data.
type TestableConsole struct{ Console }

// TestData returns sample test data.
func (x Console) TestData() (result []settings.TestBundle) {
	return
}
