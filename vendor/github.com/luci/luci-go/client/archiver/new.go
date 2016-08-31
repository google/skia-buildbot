// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package archiver

func newInt(v int) *int {
	o := new(int)
	*o = v
	return o
}

func newInt64(v int64) *int64 {
	o := new(int64)
	*o = v
	return o
}

func newString(v string) *string {
	o := new(string)
	*o = v
	return o
}
