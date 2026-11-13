// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localonly holds the checks that keep the perfinsights proof of concept on a developer
// machine: loopback-only listening and the local demo database.
package localonly

import (
	"net"
	"net/url"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// DemoDatabase is the name of the database created by perf/go/initdemo.
const DemoDatabase = "demo"

// IsLoopbackHost returns true if host is "localhost" or a loopback IP address.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// CheckDemoDatabase returns an error unless connectionString points at the database named "demo"
// on a loopback host.
//
// SECURITY: perfinsights must not read production databases until FIX-01 (Gate G1) lands, because
// sqlregression2store.GetByIDs still interpolates ids into its SQL. Errors never include the
// connection string, which may contain a password.
func CheckDemoDatabase(connectionString string) error {
	u, err := url.Parse(connectionString)
	if err != nil || (u.Scheme != "postgresql" && u.Scheme != "postgres") {
		return skerr.Fmt("the connection string must be a postgresql:// URL")
	}
	if host := u.Hostname(); !IsLoopbackHost(host) {
		return skerr.Fmt("the database host %q is not a loopback address", host)
	}
	if db := strings.TrimPrefix(u.Path, "/"); db != DemoDatabase {
		return skerr.Fmt("the database is %q, not the local %q database", db, DemoDatabase)
	}
	return nil
}
