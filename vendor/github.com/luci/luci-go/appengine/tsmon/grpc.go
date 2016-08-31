// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/tsmon/distribution"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/metric"
	"github.com/luci/luci-go/common/tsmon/types"
)

var (
	grpcServerCount = metric.NewCounter(
		"grpc/server/count",
		"Total number of RPCs.",
		types.MetricMetadata{},
		field.String("method"), // full name of the grpc method
		field.Int("code"))      // grpc.Code of the result

	grpcServerDuration = metric.NewCumulativeDistribution(
		"grpc/server/duration",
		"Distribution of server-side RPC duration (in milliseconds).",
		types.MetricMetadata{},
		distribution.DefaultBucketer,
		field.String("method"), // full name of the grpc method
		field.Int("code"))      // grpc.Code of the result
)

// NewGrpcUnaryInterceptor returns an interceptor that gathers RPC handler
// metrics and sends them to tsmon.
//
// It can be optionally chained with other interceptor. The reported metrics
// include time spent in this other interceptor too.
//
// It assumes the RPC context has tsmon initialized already (perhaps via the
// middleware).
func NewGrpcUnaryInterceptor(next grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		started := clock.Now(ctx)
		defer func() {
			reportRPCMetrics(ctx, info.FullMethod, err, clock.Now(ctx).Sub(started))
		}()
		if next != nil {
			return next(ctx, req, info, handler)
		}
		return handler(ctx, req)
	}
}

// reportRPCMetrics sends metrics after RPC handler has finished.
func reportRPCMetrics(ctx context.Context, method string, err error, dur time.Duration) {
	code := 0
	if err != nil {
		code = int(grpc.Code(err))
	}
	grpcServerCount.Add(ctx, 1, method, code)
	grpcServerDuration.Add(ctx, float64(dur.Nanoseconds()/1000000), method, code)
}
