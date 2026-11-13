// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"net/url"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cloudtop = "angdan.c.googlers.com"

func TestNewNetConfig_LoopbackByDefault(t *testing.T) {
	c, err := newNetConfig("127.0.0.1:8090", "", false, false, cloudtop)
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8090", c.PublicURL)
	assert.False(t, c.RequireAuth)
	assert.Empty(t, c.AllowedHostSuffixes)
	assert.Contains(t, c.AllowedHosts, "localhost:8090")
}

func TestNewNetConfig_NonLoopbackFailsClosed(t *testing.T) {
	c, err := newNetConfig(":8090", "", false, false, cloudtop)
	require.NoError(t, err)
	assert.True(t, c.RequireAuth)
	assert.Empty(t, c.AllowedHostSuffixes)
	assert.Equal(t, "http://angdan.c.googlers.com:8090", c.PublicURL)
}

func TestNewNetConfig_UberProxyPEN(t *testing.T) {
	c, err := newNetConfig(":8090", "", false, true, cloudtop)
	require.NoError(t, err)
	assert.False(t, c.RequireAuth)
	assert.Equal(t, []string{".proxy.googlers.com"}, c.AllowedHostSuffixes)
	assert.Equal(t, "http://angdan.c.googlers.com:8090", c.PublicURL)
	assert.Contains(t, c.AllowedHosts, "127.0.0.1:8090", "the presenter's own agent connects locally")

	_, err = newNetConfig("127.0.0.1:8090", "", false, true, cloudtop)
	assert.Error(t, err, "UberProxy can't reach loopback addresses")
	_, err = newNetConfig(":8090", "", true, true, cloudtop)
	assert.Error(t, err, "UberProxy doesn't send auth-proxy headers")
}

func TestNewNetConfig_InvalidInput(t *testing.T) {
	for _, tc := range []struct{ listen, publicURL string }{
		{"8090", ""},
		{"127.0.0.1:8090", "ftp://x.example.com"},
		{"127.0.0.1:8090", "https://x.example.com/path"},
	} {
		_, err := newNetConfig(tc.listen, tc.publicURL, false, false, cloudtop)
		assert.Error(t, err, "%+v", tc)
	}
}

func TestReadOnlyConnectionString(t *testing.T) {
	got, err := readOnlyConnectionString("postgresql://root@127.0.0.1:5433/v8_int?sslmode=disable")
	require.NoError(t, err)
	u, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "/v8_int", u.Path)
	assert.Equal(t, "root", u.User.Username())
	assert.Equal(t, "disable", u.Query().Get("sslmode"))

	// This is what the driver sends to PGAdapter when it connects.
	cfg, err := pgx.ParseConfig(got)
	require.NoError(t, err)
	assert.Equal(t, "-c default_transaction_read_only=on", cfg.RuntimeParams["options"])
	assert.Equal(t, "v8_int", cfg.Database)

	again, err := readOnlyConnectionString(got)
	require.NoError(t, err)
	assert.Equal(t, got, again, "idempotent")

	got, err = readOnlyConnectionString("postgresql://h/db?options=-c%20statement_timeout%3D5000")
	require.NoError(t, err)
	cfg, err = pgx.ParseConfig(got)
	require.NoError(t, err)
	assert.Equal(t, "-c statement_timeout=5000 -c default_transaction_read_only=on", cfg.RuntimeParams["options"], "keeps existing options")

	_, err = readOnlyConnectionString("host=127.0.0.1 dbname=demo")
	assert.Error(t, err)
}

func TestHostAllowed(t *testing.T) {
	set := toSet([]string{"127.0.0.1:8090"})
	suffixes := []string{".proxy.googlers.com"}
	for host, want := range map[string]bool{
		"127.0.0.1:8090": true,
		"b2607f8b04800100000289cb1c0a808981f90000000b690fffd5400.proxy.googlers.com":     true,
		"B2607F8B04800100000289CB1C0A808981F90000000B690FFFD5400.PROXY.GOOGLERS.COM":     true,
		"b2607f8b04800100000289cb1c0a808981f90000000b690fffd5400.proxy.googlers.com:443": true,
		"proxy.googlers.com":            false,
		"evilproxy.googlers.com":        false,
		"x.proxy.googlers.com.evil.com": false,
		"x.proxy.googlers.com/evil":     false,
		"evil.example.com":              false,
		"":                              false,
	} {
		assert.Equal(t, want, hostAllowed(host, set, suffixes), host)
	}
	assert.False(t, hostAllowed("x.proxy.googlers.com", set, nil), "suffixes are off unless configured")
}
