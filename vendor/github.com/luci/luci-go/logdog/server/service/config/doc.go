// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package config implements common LogDog daemon configuration.
//
// The majority of LogDog configuration is loaded from "luci-config". The
// specific "luci-config" instance is passed to the daemon by the LogDog
// Coordaintor service. This library implements the logic to pull the
// configuration paramerers, load the "luci-config" configuration, and expose it
// to the service daemon.
//
// Additionally, for testing, an option is implemented that allows the user to
// read the configuration from a local file (as a text protobuf) instead of
// from the Coordinator.
package config
