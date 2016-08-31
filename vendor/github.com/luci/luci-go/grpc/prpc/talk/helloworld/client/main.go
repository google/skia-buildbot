// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/luci/luci-go/examples/appengine/helloworld_standard/proto"
	"github.com/luci/luci-go/grpc/prpc"
	"golang.org/x/net/context"
)

func main() {
	ctx := context.Background()
	greeter := helloworld.NewGreeterPRPCClient(&prpc.Client{Host: "https://helloworld-dot-prpc-talk.appspot.com"})

	req := &helloworld.HelloRequest{}
	if len(os.Args) > 1 {
		req.Name = os.Args[1]
	}

	res, err := greeter.SayHello(ctx, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(res.Message)
}
