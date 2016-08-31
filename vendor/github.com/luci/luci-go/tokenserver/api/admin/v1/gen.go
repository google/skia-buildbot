// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:generate cproto -import-path=admin
//go:generate svcdec -type CertificateAuthoritiesServer
//go:generate svcdec -type ServiceAccountsServer

// Package admin contains The Token Server Administrative API.
//
// Services defined here are used by service administrators.
package admin
