// Copyright 2016 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package limitwriter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitBuf(t *testing.T) {
	buf := &bytes.Buffer{}
	b := New(buf, 10)
	n, err := b.Write([]byte("123456"))
	assert.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, 6, buf.Len())

	n, err = b.Write([]byte("123456"))
	assert.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, 10, buf.Len())
	assert.Equal(t, "1234561234", buf.String())

	n, err = b.Write([]byte("123456"))
	assert.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, 10, buf.Len())
	assert.Equal(t, "1234561234", buf.String())
}
