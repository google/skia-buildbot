// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/grpc/prpc/talk/buildbot/proto"
	"golang.org/x/net/context"
)

func main() {
	ctx := context.Background()
	greeter := buildbot.NewBuildbotPRPCClient(&prpc.Client{Host: "prpc-talk.appspot.com"})

	req := &buildbot.SearchRequest{}
	if len(os.Args) > 1 {
		req.Builder = os.Args[1]
	}

	res, err := greeter.Search(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, b := range res.Builds {
		fmt.Printf("%s: %s\n", b.Master, b.Builder)
	}
}
