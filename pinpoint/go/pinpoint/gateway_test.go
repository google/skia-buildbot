package pinpoint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func TestGetUserInfo(t *testing.T) {
	testCases := []struct {
		name          string
		ctx           context.Context
		expectedEmail string
	}{
		{
			name: "Gateway Path (HTTP)",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"grpcmetadata-x-webauth-user", "user-http@google.com",
			)),
			expectedEmail: "user-http@google.com",
		},
		{
			name: "Direct gRPC Path",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"x-webauth-user", "user-grpc@google.com",
			)),
			expectedEmail: "user-grpc@google.com",
		},
		{
			name:          "No Auth Header",
			ctx:           context.Background(),
			expectedEmail: "",
		},
	}

	srv := &gatewayServer{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.GetUserInfo(tc.ctx, &pb.GetUserInfoRequest{})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedEmail, resp.Email)
		})
	}
}
