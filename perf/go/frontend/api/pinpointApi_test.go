package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	aloginMocks "go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/kube/go/authproxy"
	"go.skia.org/infra/pinpoint/go/pinpoint"
	"google.golang.org/grpc/metadata"
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

	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"sruslan@google.com"}, md.Get(authproxy.WebAuthHeaderName))
	require.Equal(t, []string{"bisecter"}, md.Get(authproxy.WebAuthRoleHeaderName))
}
