// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package collector implements the LogDog Collector daemon's log parsing and
// registration logic.
//
// The LogDog Collector is responsible for ingesting logs from the Butler sent
// via transport, stashing them in intermediate storage, and registering them
// with the LogDog Coordinator service.
package collector
