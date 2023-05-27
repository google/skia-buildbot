package grpclogging

import (
	"bytes"
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/go/now"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	startTime = time.Date(2022, time.January, 31, 2, 2, 3, 0, time.FixedZone("UTC+1", 60*60))
)

func testSetupLogger(t *testing.T) (*now.TimeTravelCtx, *GRPCLogger, *bytes.Buffer) {
	ttCtx := now.TimeTravelingContext(startTime)
	buf := &bytes.Buffer{}
	l := New(buf)

	return ttCtx, l, buf
}

func TestServerUnaryLoggingInterceptor_noError(t *testing.T) {
	ttCtx, l, buf := testSetupLogger(t)
	req := &cpb.GetAnalysisRequest{
		PinpointJobId: proto.String("d3c4f84d"),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		ttCtx.SetTime(startTime.Add(3 * time.Second))
		return &cpb.GetAnalysisResponse{}, nil
	}
	resp, err := l.ServerUnaryLoggingInterceptor(ttCtx, req, &grpc.UnaryServerInfo{FullMethod: "test.service/TestMethod"}, handler)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t,
		`{"start":"2022-01-31T02:02:03+01:00","elapsed_ns":3000000000,"server_unary":{"request":{"pinpoint_job_id":"d3c4f84d"},"response":{},"full_method":"test.service/TestMethod"}}`+"\n",
		string(buf.Bytes()))
}

func TestServerUnaryLoggingInterceptor_error(t *testing.T) {
	ttCtx, l, buf := testSetupLogger(t)
	req := &cpb.GetAnalysisRequest{
		PinpointJobId: proto.String("d3c4f84d"),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		ttCtx.SetTime(startTime.Add(1 * time.Second))
		return nil, status.Errorf(codes.InvalidArgument, "forced error response")
	}
	resp, err := l.ServerUnaryLoggingInterceptor(ttCtx, req, &grpc.UnaryServerInfo{FullMethod: "test.service/TestMethod"}, handler)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t,
		`{"start":"2022-01-31T02:02:03+01:00","elapsed_ns":1000000000,"status":{"code":3,"message":"forced error response"},"error":"rpc error: code = InvalidArgument desc = forced error response","server_unary":{"request":{"pinpoint_job_id":"d3c4f84d"},"full_method":"test.service/TestMethod"}}`+"\n",
		string(buf.Bytes()))
}

func TestClientUnaryLoggingInterceptor_noError(t *testing.T) {
	ttCtx, l, buf := testSetupLogger(t)
	req := &cpb.GetAnalysisRequest{
		PinpointJobId: proto.String("d3c4f84d"),
	}
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		ttCtx.SetTime(startTime.Add(3 * time.Second))
		pb, _ := reply.(*cpb.GetAnalysisResponse)
		pb.Results = []*cpb.AnalysisResult{}

		return nil
	}
	resp := &cpb.GetAnalysisResponse{}
	err := l.ClientUnaryLoggingInterceptor(ttCtx, "test.service/TestMethod", req, resp, nil, invoker, nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t,
		`{"start":"2022-01-31T02:02:03+01:00","elapsed_ns":3000000000,"client_unary":{"method":"test.service/TestMethod","request":{"pinpoint_job_id":"d3c4f84d"},"response":{}}}`+"\n",
		string(buf.Bytes()))
}

func TestClientUnaryLoggingInterceptor_error(t *testing.T) {
	ttCtx, l, buf := testSetupLogger(t)
	req := &cpb.GetAnalysisRequest{
		PinpointJobId: proto.String("d3c4f84d"),
	}
	invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		ttCtx.SetTime(startTime.Add(3 * time.Second))

		return status.Errorf(codes.InvalidArgument, "forced error response")
	}
	resp := &cpb.GetAnalysisResponse{}
	err := l.ClientUnaryLoggingInterceptor(ttCtx, "test.service/TestMethod", req, resp, nil, invoker, nil)
	require.Error(t, err)
	assert.Equal(t,
		`{"start":"2022-01-31T02:02:03+01:00","elapsed_ns":3000000000,"status":{"code":3,"message":"forced error response"},"error":"rpc error: code = InvalidArgument desc = forced error response","client_unary":{"method":"test.service/TestMethod","request":{"pinpoint_job_id":"d3c4f84d"},"response":{}}}`+"\n",
		string(buf.Bytes()))
}

func TestClientStreamLoggingInterceptor_noError(t *testing.T) {
	ttCtx, l, buf := testSetupLogger(t)
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ttCtx.SetTime(startTime.Add(3 * time.Second))

		return nil, nil
	}
	desc := &grpc.StreamDesc{
		StreamName: "test/stream",
	}
	_, err := l.ClientStreamLoggingInterceptor(ttCtx, desc, nil, "test.service/TestMethod", streamer)
	require.NoError(t, err)
	assert.Equal(t,
		`{"start":"2022-01-31T02:02:03+01:00","elapsed_ns":3000000000,"client_stream":{"method":"test.service/TestMethod","stream_name":"test/stream"}}`+"\n",
		string(buf.Bytes()))
}

func TestClientStreamLoggingInterceptor_error(t *testing.T) {
	ttCtx, l, buf := testSetupLogger(t)
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ttCtx.SetTime(startTime.Add(3 * time.Second))

		return nil, status.Errorf(codes.InvalidArgument, "forced error response")
	}
	desc := &grpc.StreamDesc{
		StreamName: "test/stream",
	}
	_, err := l.ClientStreamLoggingInterceptor(ttCtx, desc, nil, "test.service/TestMethod", streamer)
	require.Error(t, err)
	assert.Equal(t,
		`{"start":"2022-01-31T02:02:03+01:00","elapsed_ns":3000000000,"status":{"code":3,"message":"forced error response"},"error":"rpc error: code = InvalidArgument desc = forced error response","client_stream":{"method":"test.service/TestMethod","stream_name":"test/stream"}}`+"\n",
		string(buf.Bytes()))
}
