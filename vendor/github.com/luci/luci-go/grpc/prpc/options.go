// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/luci/luci-go/common/retry"
)

// Options controls how RPC requests are sent.
type Options struct {
	Retry retry.Factory // RPC retrial.

	// UserAgent is the value of User-Agent HTTP header.
	// If empty, DefaultUserAgent is used.
	UserAgent string
	Insecure  bool // if true, use HTTP instead of HTTPS.

	// the rest can be set only using CallOption.

	resHeaderMetadata  *metadata.MD // destination for response HTTP headers.
	resTrailerMetadata *metadata.MD // destination for response HTTP trailers.
	serverDeadline     time.Duration
}

// DefaultOptions are used if no options are specified in Client.
func DefaultOptions() *Options {
	return &Options{
		Retry: func() retry.Iterator {
			return &retry.ExponentialBackoff{
				Limited: retry.Limited{
					Delay:   time.Second,
					Retries: 5,
				},
			}
		},
	}
}

func (o *Options) apply(callOptions []grpc.CallOption) error {
	for _, co := range callOptions {
		prpcCo, ok := co.(*CallOption)
		if !ok {
			return fmt.Errorf("non-pRPC call option %T is used with pRPC client", co)
		}
		prpcCo.apply(o)
	}
	return nil
}

// CallOption mutates Options.
type CallOption struct {
	grpc.CallOption
	// apply mutates options.
	apply func(*Options)
}

// Header returns a CallOption that retrieves the header metadata.
// Can be used instead of with grpc.Header.
func Header(md *metadata.MD) *CallOption {
	return &CallOption{
		grpc.Header(md),
		func(o *Options) {
			o.resHeaderMetadata = md
		},
	}
}

// Trailer returns a CallOption that retrieves the trailer metadata.
// Can be used instead of grpc.Trailer.
func Trailer(md *metadata.MD) *CallOption {
	return &CallOption{
		grpc.Trailer(md),
		func(o *Options) {
			o.resTrailerMetadata = md
		},
	}
}
