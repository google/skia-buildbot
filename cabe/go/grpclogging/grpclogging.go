// Package grpclogging provides client and sever interceptors to log grpc requests, responses,
// errors and other metadata which is helpful for debugging and analysis.  This package assumes
// the caller is running in a skia-infra managed GKE cluster, such that stdout is parsed as
// newline-delimited json and passed to StackDriver logging for storage.
package grpclogging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy"
)

type grpcLogEntry struct {
	Start        time.Time                 `json:"start,omitempty"`
	Elapsed      time.Duration             `json:"elapsed_ns,omitempty"`
	Status       *spb.Status               `json:"status,omitempty"`
	Error        string                    `json:"error,omitempty"`
	err          error                     `json:"omit"`
	ServerUnary  *serverUnaryGRPCLogEntry  `json:"server_unary,omitempty"`
	ClientUnary  *clientUnaryGRPCLogEntry  `json:"client_unary,omitempty"`
	ClientStream *clientStreamGRPCLogEntry `json:"client_stream,omitempty"`
}

type serverUnaryGRPCLogEntry struct {
	Request    any    `json:"request,omitempty"`
	Response   any    `json:"response,omitempty"`
	FullMethod string `json:"full_method,omitempty"`
	User       string `json:"user"`
}

type clientUnaryGRPCLogEntry struct {
	Method   string `json:"method,omitempty"`
	Request  any    `json:"request,omitempty"`
	Response any    `json:"response,omitempty"`
}

type clientStreamGRPCLogEntry struct {
	Method        string `json:"method,omitempty"`
	StreamName    string `json:"stream_name,omitempty"`
	ServerStreams bool   `json:"server_streams,omitempty"`
	ClientStreams bool   `json:"client_streams,omitempty"`
}

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
	resp, err := handler(ctx, req)
	elapsed := now.Now(ctx).Sub(start)
	l.log(&grpcLogEntry{
		Start:   start,
		Elapsed: elapsed,
		ServerUnary: &serverUnaryGRPCLogEntry{
			Request:    req,
			Response:   resp,
			FullMethod: info.FullMethod,
			User:       authProxyUser,
		},
		err: err,
	})

	return resp, err
}

// ClientUnaryLoggingInterceptor implements [grpc.UnaryClientInterceptor].
func (l *GRPCLogger) ClientUnaryLoggingInterceptor(ctx context.Context, method string, req, resp any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	start := now.Now(ctx)
	err := invoker(ctx, method, req, resp, cc, opts...)
	elapsed := now.Now(ctx).Sub(start)
	l.log(&grpcLogEntry{
		Start:   start,
		Elapsed: elapsed,
		ClientUnary: &clientUnaryGRPCLogEntry{
			Method:   method,
			Request:  req,
			Response: resp,
		},
		err: err,
	})

	return err
}

// ClientStreamLoggingInterceptor implements [grpc.StreamClientInterceptor].
func (l *GRPCLogger) ClientStreamLoggingInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	start := now.Now(ctx)
	clientStream, err := streamer(ctx, desc, cc, method, opts...)
	elapsed := now.Now(ctx).Sub(start)
	l.log(&grpcLogEntry{
		Start:   start,
		Elapsed: elapsed,
		ClientStream: &clientStreamGRPCLogEntry{
			Method:        method,
			StreamName:    desc.StreamName,
			ServerStreams: desc.ServerStreams,
			ClientStreams: desc.ClientStreams,
		},
		err: err,
	})

	return clientStream, err
}

func (l *GRPCLogger) log(entry *grpcLogEntry) {
	if st, ok := status.FromError(entry.err); ok {
		entry.Status = st.Proto()
	}
	if entry.err != nil {
		entry.Error = entry.err.Error()
	}

	b, err := json.Marshal(entry)
	if err != nil {
		sklog.Errorf("Failed to marshal grpc log entry: %s", err)
	} else {
		_, err := fmt.Fprintln(l.out, string(b))
		if err != nil {
			sklog.Errorf("writing out grpc logger: %v", err)
		}
	}
}
