package pinpoint

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestNewGatewayJSONHandler_GetUserInfo(t *testing.T) {
	ctx := context.Background()
	handler, err := NewGatewayJSONHandler(ctx, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/pinpoint/v1/user", http.NoBody)
	req.Header.Set("X-WEBAUTH-USER", "user@google.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"email":"user@google.com"`)
}
