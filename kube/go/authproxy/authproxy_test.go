package authproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/kube/go/authproxy/auth/mocks"
)

const email = "nobody@example.org"

func setupForTest(t *testing.T, cb http.HandlerFunc) (*url.URL, *bool, *httptest.ResponseRecorder, *http.Request) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cb(w, r)
		called = true
	}))
	t.Cleanup(func() {
		ts.Close()
	})
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", ts.URL, nil)
	return u, &called, w, r
}

func TestProxyServeHTTP_AllowPostAndNotAuthenticated_WebAuthHeaderValueIsEmptyString(t *testing.T) {

	u, called, w, r := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		// Note that if the header webAuthHeaderName hadn't been set then the value would be nil.
		require.Equal(t, []string{""}, r.Header.Values(webAuthHeaderName))
		require.Equal(t, []string(nil), r.Header.Values("X-SOME-UNSET-HEADER"))
	})
	authMock := &mocks.Auth{}
	authMock.On("LoggedInAs", r).Return("")

	proxy := newProxy(u, authMock, true, false)

	proxy.ServeHTTP(w, r)
	require.True(t, *called)
	authMock.AssertExpectations(t)
}

func TestProxyServeHTTP_UserIsLoggedIn_HeaderWithUserEmailIsIncludedInRequest(t *testing.T) {

	u, called, w, r := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, []string{email}, r.Header.Values(webAuthHeaderName))
	})

	authMock := &mocks.Auth{}
	authMock.On("LoggedInAs", r).Return(email)
	authMock.On("IsViewer", r).Return(true)

	proxy := newProxy(u, authMock, false, false)

	proxy.ServeHTTP(w, r)
	require.True(t, *called)
	authMock.AssertExpectations(t)
}

func TestProxyServeHTTP_UserIsNotLoggedIn_HeaderWithUserEmailIsStrippedFromRequest(t *testing.T) {

	u, called, w, r := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {})
	r.Header.Add(webAuthHeaderName, email) // Try to spoof the header.
	authMock := &mocks.Auth{}
	authMock.On("LoggedInAs", r).Return("")
	authMock.On("LoginURL", w, r).Return("http://example.org/login")

	proxy := newProxy(u, authMock, false, false)

	proxy.ServeHTTP(w, r)
	require.False(t, *called)
	authMock.AssertExpectations(t)
}

func TestProxyServeHTTP_UserIsLoggedInButNotAViewer_ReturnsStatusForbidden(t *testing.T) {

	u, called, w, r := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {})
	authMock := &mocks.Auth{}
	authMock.On("LoggedInAs", r).Return(email)
	authMock.On("IsViewer", r).Return(false)

	proxy := newProxy(u, authMock, false, false)

	proxy.ServeHTTP(w, r)
	require.False(t, *called)
	require.Equal(t, http.StatusForbidden, w.Result().StatusCode)
	authMock.AssertExpectations(t)
}

func TestProxyServeHTTP_UserIsLoggedIn_HeaderWithUserEmailIsIncludedInRequestAndSpoofedEmailIsRemoved(t *testing.T) {

	u, called, w, r := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, []string{email}, r.Header.Values(webAuthHeaderName))
	})
	r.Header.Add(webAuthHeaderName, "haxor@example.org") // Try to spoof the header.
	authMock := &mocks.Auth{}
	authMock.On("LoggedInAs", r).Return(email)
	authMock.On("IsViewer", r).Return(true)

	proxy := newProxy(u, authMock, false, false)

	proxy.ServeHTTP(w, r)
	require.True(t, *called)
	authMock.AssertExpectations(t)
}

func TestProxyServeHTTP_UserIsNotLoggedInAndPassiveFlagIsSet_RequestIsPassedAlongWithoutEmailHeader(t *testing.T) {

	u, called, w, r := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, []string{""}, r.Header.Values(webAuthHeaderName))
	})

	r.Header.Add(webAuthHeaderName, "haxor@example.org") // Try to spoof the header.
	authMock := &mocks.Auth{}
	authMock.On("LoggedInAs", r).Return("")

	proxy := newProxy(u, authMock, false, true)

	proxy.ServeHTTP(w, r)
	require.True(t, *called)
	authMock.AssertExpectations(t)
}

func TestProxyServeHTTP_UserIsLoggedInAndPassiveFlagIsSet_RequestIsPassedAlongWithEmailHeader(t *testing.T) {

	u, called, w, r := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, []string{email}, r.Header.Values(webAuthHeaderName))
	})

	r.Header.Add(webAuthHeaderName, "haxor@example.org") // Try to spoof the header.
	authMock := &mocks.Auth{}
	authMock.On("LoggedInAs", r).Return(email)

	proxy := newProxy(u, authMock, false, true)

	proxy.ServeHTTP(w, r)
	require.True(t, *called)
	authMock.AssertExpectations(t)
}

func TestValidateFlags_BothFlagsSpecified_ReturnsError(t *testing.T) {
	app := &App{
		criaGroup:   "project-angle-committers",
		allowedFrom: "google.com",
	}

	require.Error(t, app.validateFlags())
}

func TestValidateFlags_NeitherFlagIsSpecified_ReturnsError(t *testing.T) {
	app := &App{
		criaGroup:   "",
		allowedFrom: "",
	}

	require.Error(t, app.validateFlags())
}

func TestValidateFlags_OnlyOneFlagIsSpecified_ReturnsNoError(t *testing.T) {

	app := &App{
		criaGroup:   "project-angle-committers",
		allowedFrom: "",
	}

	require.NoError(t, app.validateFlags())

	app = &App{
		criaGroup:   "",
		allowedFrom: "google.com",
	}

	require.NoError(t, app.validateFlags())
}

func TestAppRun_ContextIsCancelled_ReturnsNil(t *testing.T) {
	// Construct minimal App.
	target, err := url.Parse("http://my-service")
	require.NoError(t, err)
	app := &App{
		target:   target,
		port:     ":0",
		promPort: ":0",
	}
	app.registerCleanup()

	var w sync.WaitGroup
	w.Add(1)
	go func() {
		err := app.Run(context.Background())
		assert.NoError(t, err)
		w.Done()
	}()

	// Ensure the server has been started.
	for app.server == nil {
		time.Sleep(time.Millisecond)
	}

	// Force a cleanup.
	cleanup.Cleanup()
	w.Wait()

	// Test will fail by timeout if the app.Run() didn't return.
}
