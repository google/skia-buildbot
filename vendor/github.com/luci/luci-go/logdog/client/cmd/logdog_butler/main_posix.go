// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/luci/luci-go/logdog/client/butler/streamserver"
	"golang.org/x/net/context"
)

const (
	// An example stream server URI.
	exampleStreamServerURI = streamServerURI("unix:/var/run/butler.sock")
)

// interruptSignals is the set of signals to handle gracefully (e.g., flush,
// shutdown).
var interruptSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
}

type streamServerURI string

func (u streamServerURI) Parse() (string, error) {
	typ, value := parseStreamServer(string(u))
	if typ != "unix" {
		return "", fmt.Errorf("unsupported URI scheme: [%s]", typ)
	}
	if value == "" {
		return "", errors.New("empty stream server path")
	}
	return value, nil
}

// Validates that the URI is correct for Windows.
func (u streamServerURI) Validate() (err error) {
	_, err = u.Parse()
	return
}

// Create a POSIX (UNIX named pipe) stream server
func createStreamServer(ctx context.Context, uri streamServerURI) streamserver.StreamServer {
	path, err := uri.Parse()
	if err != nil {
		panic("Failed to parse stream server URI.")
	}
	return streamserver.NewNamedPipeServer(ctx, path)
}
