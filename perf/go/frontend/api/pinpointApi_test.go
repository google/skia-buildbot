package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	aloginMocks "go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/kube/go/authproxy"
	"go.skia.org/infra/kube/go/authproxy/protoheader"
	"go.skia.org/infra/pinpoint/go/pinpoint"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

func TestCreateTryJob_NotLoggedIn(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	// HasRole returns false to mock unauthorized user
	login.On("HasRole", mock.Anything, mock.Anything).Return(false)

	// Instantiate pinpointApi
	api := NewPinpointApi(login, &pinpoint.Client{}, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/try/", nil)

	api.createTryJobHandler(w, r)

	require.Equal(http.StatusForbidden, w.Code)
	require.Contains(w.Body.String(), "User is not logged in or is not authorized")
	login.AssertExpectations(t)
}

func TestGetContextWithAuthHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/_/bisect/", nil)
	r.Header.Set(authproxy.WebAuthHeaderName, "sruslan@google.com")
	r.Header.Set(authproxy.WebAuthRoleHeaderName, "bisecter")
	r.Header.Set(authproxy.EndpointAPIUserInfoHeaderName, "base64_proto_data")
	r.Header.Set(authproxy.GoogAuthenticatedUserEmailHeaderName, "accounts.google.com:sruslan@google.com")

	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"sruslan@google.com"}, md.Get(authproxy.WebAuthHeaderName))
	require.Equal(t, []string{"bisecter"}, md.Get(authproxy.WebAuthRoleHeaderName))
	require.Equal(t, []string{"base64_proto_data"}, md.Get(authproxy.EndpointAPIUserInfoHeaderName))
	require.Equal(t, []string{"accounts.google.com:sruslan@google.com"}, md.Get(authproxy.GoogAuthenticatedUserEmailHeaderName))
}

func TestGetContextWithAuthHeaders_SynthesizesMissingHeadersFromGoogUser(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/_/bisect/", nil)
	r.Header.Set(authproxy.GoogAuthenticatedUserEmailHeaderName, "accounts.google.com:sruslan@google.com")
	r.Header.Set(authproxy.WebAuthRoleHeaderName, "bisecter")

	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"sruslan@google.com"}, md.Get(authproxy.WebAuthHeaderName))
	require.Equal(t, []string{"bisecter"}, md.Get(authproxy.WebAuthRoleHeaderName))
	require.Equal(t, []string{"accounts.google.com:sruslan@google.com"}, md.Get(authproxy.GoogAuthenticatedUserEmailHeaderName))

	epUserList := md.Get(authproxy.EndpointAPIUserInfoHeaderName)
	require.Len(t, epUserList, 1)
	epUser := epUserList[0]

	// Simulate ProtoHeader.LoggedInAs decoding logic on perf-be
	reqForBe := httptest.NewRequest(http.MethodPost, "/pinpoint.v1.Pinpoint/SchedulePairwise", nil)
	reqForBe.Header.Set("X-Endpoint-API-UserInfo", epUser)

	headerValue := reqForBe.Header.Get("X-Endpoint-API-UserInfo")
	parts := strings.Split(headerValue, ".")
	require.Len(t, parts, 2)

	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)

	var h protoheader.Header
	err = proto.Unmarshal(b, &h)
	require.NoError(t, err)
	require.Equal(t, "sruslan@google.com", h.Email)
}

func TestGetContextWithAuthHeaders_SynthesizesMissingHeadersFromWebAuthUser(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/_/bisect/", nil)
	r.Header.Set(authproxy.WebAuthHeaderName, "sruslan@google.com")

	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"sruslan@google.com"}, md.Get(authproxy.WebAuthHeaderName))
	require.Equal(t, []string{"accounts.google.com:sruslan@google.com"}, md.Get(authproxy.GoogAuthenticatedUserEmailHeaderName))

	epUserList := md.Get(authproxy.EndpointAPIUserInfoHeaderName)
	require.Len(t, epUserList, 1)
	epUser := epUserList[0]

	reqForBe := httptest.NewRequest(http.MethodPost, "/pinpoint.v1.Pinpoint/SchedulePairwise", nil)
	reqForBe.Header.Set("X-Endpoint-API-UserInfo", epUser)

	headerValue := reqForBe.Header.Get("X-Endpoint-API-UserInfo")
	parts := strings.Split(headerValue, ".")
	require.Len(t, parts, 2)

	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)

	var h protoheader.Header
	err = proto.Unmarshal(b, &h)
	require.NoError(t, err)
	require.Equal(t, "sruslan@google.com", h.Email)
}
