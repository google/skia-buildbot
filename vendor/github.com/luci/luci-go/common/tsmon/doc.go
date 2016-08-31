// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package tsmon contains global state and utility functions for configuring
// and interacting with tsmon.  This has a similar API to the infra-python
// ts_mon module.
//
// Users of tsmon should call InitializeFromFlags from their application's main
// function.
package tsmon
