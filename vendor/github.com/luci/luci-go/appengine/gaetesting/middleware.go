// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gaetesting

import (
	"golang.org/x/net/context"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/server/secrets/testsecrets"
)

// TestingContext returns context with base services installed:
//   * github.com/luci/gae/impl/memory (in-memory appengine services)
//   * github.com/luci/luci-go/server/secrets/testsecrets (access to fake secret keys)
func TestingContext() context.Context {
	c := context.Background()
	c = memory.Use(c)
	c = testsecrets.Use(c)
	c = proccache.Use(c, &proccache.Cache{})
	return c
}
