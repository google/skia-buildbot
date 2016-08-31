// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package main contains the entry point code for the LUCI Deployment Tool
// ("luci_deploy"). This tool is a command-line interface designed to perform
// controlled and automated deployment of LUCI (and LUCI-compatible) services.
//
// "luci_deploy" is Linux-oriented, but may also work on other platforms. It
// leverages external tooling for many remote operations; it is the
// responsibility of the user to have suitable versions of that tooling
// installed and available on PATH.
package main
