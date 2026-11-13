// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package localonly

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLoopbackHost(t *testing.T) {
	for _, h := range []string{"localhost", "LOCALHOST", "127.0.0.1", "127.0.0.2", "::1"} {
		assert.True(t, IsLoopbackHost(h), h)
	}
	for _, h := range []string{"", "0.0.0.0", "::", "10.0.0.1", "example.com", "localhost.example.com"} {
		assert.False(t, IsLoopbackHost(h), h)
	}
}

func TestCheckDemoDatabase(t *testing.T) {
	for _, ok := range []string{
		"postgresql://root@127.0.0.1:5432/demo?sslmode=disable",
		"postgresql://root@localhost:5432/demo?sslmode=disable",
		"postgres://[::1]:5432/demo",
	} {
		assert.NoError(t, CheckDemoDatabase(ok), ok)
	}
	for _, bad := range []string{
		"",
		"mysql://127.0.0.1/demo",
		"postgresql://root@10.1.2.3:5432/demo?sslmode=disable",
		"postgresql://root@127.0.0.1:5432/v8_int?sslmode=disable",
		"postgresql://root@127.0.0.1:5432/?sslmode=disable",
	} {
		assert.Error(t, CheckDemoDatabase(bad), bad)
	}
}

func TestCheckDemoDatabase_DoesNotLeakPasswords(t *testing.T) {
	err := CheckDemoDatabase("postgresql://user:hunter2@10.1.2.3:5432/prod")
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
}
