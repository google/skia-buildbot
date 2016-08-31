// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"os"

	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/luci/luci-go/tools/internal/apigen"
	"golang.org/x/net/context"
)

func main() {
	a := apigen.Application{}
	lc := log.Config{
		Level: log.Warning,
	}

	fs := flag.CommandLine
	a.AddToFlagSet(fs)
	lc.AddFlags(fs)
	fs.Parse(os.Args[1:])

	ctx := context.Background()
	ctx = lc.Set(gologger.StdConfig.Use(ctx))

	if err := a.Run(ctx); err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(ctx, "An error occurred during execution.")
		os.Exit(1)
	}
}
