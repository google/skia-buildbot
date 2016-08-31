// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package client

import (
	"net/http"

	"github.com/luci/luci-go/common/retry"
	"github.com/luci/luci-go/grpc/prpc"

	"github.com/luci/luci-go/tokenserver/api/minter/v1"
)

// Parameters is passed to New.
type Parameters struct {
	// PrivateKeyPath is a path to a file with a private key PEM file.
	//
	// Required.
	PrivateKeyPath string

	// CertificatePath is a path to a file with a corresponding certificate.
	//
	// Required. It must match the private key (this will be verified).
	CertificatePath string

	// Backend is a hostname of the token server to talk to.
	//
	// Required.
	Backend string

	// Insecure is true to use 'http' protocol instead of 'https'.
	//
	// Useful on localhost. Default is "secure".
	Insecure bool

	// Client is non-authenticating HTTP client to build pRPC transport on top of.
	//
	// Default is http.DefaultClient.
	Client *http.Client

	// Retry defines how to retry RPC requests on transient errors.
	//
	// Use retry.Default for default strategy. Default is "no retries".
	Retry retry.Factory
}

// New returns new Client that uses PEM encoded keys and talks
// to the server via pRPC.
func New(params Parameters) (*Client, error) {
	signer, err := LoadX509Signer(params.PrivateKeyPath, params.CertificatePath)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client: minter.NewTokenMinterPRPCClient(&prpc.Client{
			C:    params.Client,
			Host: params.Backend,
			Options: &prpc.Options{
				Retry:    params.Retry,
				Insecure: params.Insecure,
			},
		}),
		Signer: signer,
	}, nil
}
