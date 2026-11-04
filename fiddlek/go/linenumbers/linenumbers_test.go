// Copyright 2016 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package linenumbers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLineNumbers(t *testing.T) {
	code := `a
b
c`
	want := `#line 1
a
b
c`
	assert.Equal(t, want, LineNumbers(code))

	code = ``
	want = `#line 1
`
	assert.Equal(t, want, LineNumbers(code))
}
