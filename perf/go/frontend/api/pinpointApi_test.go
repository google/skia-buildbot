package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	aloginMocks "go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/pinpoint/go/pinpoint"
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
