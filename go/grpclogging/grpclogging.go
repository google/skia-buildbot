// Package grpclogging provides client and server interceptors to log grpc requests, responses,
// errors and other metadata which is helpful for debugging and analysis.  This package assumes
// the caller is running in a skia-infra managed GKE cluster, such that stdout is parsed as
// newline-delimited json and passed to StackDriver logging for storage.
package grpclogging

import (
	"context"
	"fmt"
	"io"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "go.skia.org/infra/go/grpclogging/proto"
)

// GRPCLogger provides interceptor methods for grpc clients and servers to log the request
// and response activity going through them.
type GRPCLogger struct {
	out io.Writer
}

// New returns a new GRPCLogger instance that will write json-encoded log lines to w.
func New(w io.Writer) *GRPCLogger {
	return &GRPCLogger{
		out: w,
	}
}

func userFromAuthProxy(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	user := md.Get(authproxy.WebAuthHeaderName)
	if len(user) > 0 {
		return user[0]
	}
	return ""
}

// ServerUnaryLoggingInterceptor implements [grpc.UnaryServerInterceptor].
func (l *GRPCLogger) ServerUnaryLoggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	authProxyUser := userFromAuthProxy(ctx)
	start := now.Now(ctx)
	startPb := timestamppb.New(start)

	resp, handlerErr := handler(ctx, req)
	elapsed := durationpb.New(now.Now(ctx).Sub(start))

	reqAny, err := anypb.New(req.(proto.Message))
	if err != nil {
		sklog.Errorf("ServerUnaryLoggingInterceptor couldn't log request: %v", err)
	}
	entry := &pb.Entry{
		Start:   startPb,
		Elapsed: elapsed,
		ServerUnary: &pb.ServerUnary{
			Request:    reqAny,
			FullMethod: info.FullMethod,
			User:       authProxyUser,
		},
	}
	if resp != nil {
		respAny, err := anypb.New(resp.(proto.Message))
		if err != nil {
			sklog.Errorf("ServerUnaryLoggingInterceptor couldn't log response: %v", err)
		} else {
			entry.ServerUnary.Response = respAny
		}
	}
	l.log(entry, handlerErr)

	return resp, handlerErr
}

// ClientUnaryLoggingInterceptor implements [grpc.UnaryClientInterceptor].
func (l *GRPCLogger) ClientUnaryLoggingInterceptor(ctx context.Context, method string, req, resp any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	start := now.Now(ctx)
	startPb := timestamppb.New(start)
	invokerErr := invoker(ctx, method, req, resp, cc, opts...)
	elapsed := durationpb.New(now.Now(ctx).Sub(start))

	reqAny, err := anypb.New(req.(proto.Message))
	if err != nil {
		sklog.Errorf("ClientUnaryLoggingInterceptor couldn't log request: %v", err)
	}
	respAny, err := anypb.New(resp.(proto.Message))
	if err != nil {
		sklog.Errorf("ClientUnaryLoggingInterceptor couldn't log response: %v", err)
	}
	l.log(&pb.Entry{
		Start:   startPb,
		Elapsed: elapsed,
		ClientUnary: &pb.ClientUnary{
			Method:   method,
			Request:  reqAny,
			Response: respAny,
		},
	}, invokerErr)

	return invokerErr
}

// ClientStreamLoggingInterceptor implements [grpc.StreamClientInterceptor].
func (l *GRPCLogger) ClientStreamLoggingInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	start := now.Now(ctx)
	startPb := timestamppb.New(start)

	clientStream, streamerErr := streamer(ctx, desc, cc, method, opts...)
	elapsed := durationpb.New(now.Now(ctx).Sub(start))

	l.log(&pb.Entry{
		Start:   startPb,
		Elapsed: elapsed,
		ClientStream: &pb.ClientStream{
			Method:        method,
			StreamName:    desc.StreamName,
			ServerStreams: desc.ServerStreams,
			ClientStreams: desc.ClientStreams,
		},
	}, streamerErr)

	return clientStream, streamerErr
}

func (l *GRPCLogger) log(entry *pb.Entry, err error) {
	if st, ok := status.FromError(err); ok {
		statusProto := st.Proto()
		if statusProto != nil {
			entry.StatusCode = statusProto.Code
			entry.StatusMessage = statusProto.Message
			entry.StatusDetails = statusProto.Details
		}
	}
	if err != nil {
		entry.Error = err.Error()
	}

	b, err := protojson.MarshalOptions{
		Multiline:      false,
		Indent:         "",
		AllowPartial:   true,
		UseProtoNames:  true,
		UseEnumNumbers: false,
	}.Marshal(entry)
	if err != nil {
		sklog.Errorf("Failed to marshal grpc log entry: %s", err)
	} else {
		_, err := fmt.Fprintln(l.out, string(b))
		if err != nil {
			sklog.Errorf("writing out grpc logger: %v", err)
		}
	}
}
