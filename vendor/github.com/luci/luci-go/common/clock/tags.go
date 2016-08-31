// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package clock

import (
	"golang.org/x/net/context"
)

var clockTagKey = "clock-tag"

// Tag returns a derivative Context with the supplied Tag appended to it.
//
// Tag chains can be used by timers to identify themselves.
func Tag(c context.Context, v string) context.Context {
	return context.WithValue(c, &clockTagKey, append(Tags(c), v))
}

// Tags returns a copy of the set of tags in the current Context.
func Tags(c context.Context) []string {
	if tags, ok := c.Value(&clockTagKey).([]string); ok && len(tags) > 0 {
		tclone := make([]string, len(tags))
		copy(tclone, tags)
		return tclone
	}
	return nil
}
