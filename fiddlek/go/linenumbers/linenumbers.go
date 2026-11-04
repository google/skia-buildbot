// Copyright 2016 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package linenumbers

// LineNumbers adds #line numbering to the user's code.
func LineNumbers(c string) string {
	return "#line 1\n" + c
}
