// Package oauth2redirect is a reverse proxy that runs in front of applications and
// takes care of handling the oauth2 redirect leg of the OAuth 3-legged flow. It passes
// all other traffic to the application it is running in front of.
//
// This is useful so that don't need to redeploy docsyserver everytime a change
// is made to //go/login, instead just this smaller proxy can be deployed at the same
// time as go/auth-proxy is deployed.
package oauth2redirect

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/login"
)

const (
	helloWorld = "Hello World"
)

func setupForTest(t *testing.T) *App {
	// Create a server to run behind the proxy that always returns helloWorld.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEqual(t, r.URL.Path, "/oauth2callback/")
		_, err := w.Write([]byte(helloWorld))
		require.NoError(t, err)
	}))

	// Swap out os.Args since we are calling New() which parses flags.
	backupArgs := append([]string{}, os.Args...)
	t.Cleanup(func() {
		os.Args = backupArgs
	})

	os.Args = []string{"oauth2redirect", "--port=:0", "--prom-port=:0", fmt.Sprintf("--target_port=%s", testServer.URL)}

	// Create a new App.
	a, err := New(context.Background(), login.SkipLoadingSecrets{})
	require.NoError(t, err)

	// Start the App HTTP server.
	go func() {
		err := a.Run(context.Background())
		require.NoError(t, err)
	}()

	// Wait for Run() to initialize the proxy.
	for a.proxy == nil {
		time.Sleep(time.Millisecond)
	}

	return a
}

func TestApp_RequestToOAuth2CallbackPath_ProxyInterceptsRequest(t *testing.T) {
	a := setupForTest(t)

	testCases := []struct {
		url         string
		statusCode  int
		body        string
		subTestName string
	}{
		{
			subTestName: "proxy intercepts request to /oauth2callback/",
			url:         login.DefaultOAuth2Callback,
			statusCode:  500,
			body:        "Missing session state",
		},
		{
			subTestName: "proxy intercepts request to /login/ which redirects to first request of 3-legged flow",
			url:         login.LoginPath,
			statusCode:  302,
			body:        `<a href="https://`,
		},
		{
			subTestName: "proxy intercepts request to /logout/, clears cookie, and redirects to /.",
			url:         login.LogoutPath,
			statusCode:  302,
			body:        `<a href="/"`,
		},
		{
			subTestName: "all other requests pass through the proxy",
			url:         "/",
			statusCode:  200,
			body:        helloWorld,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.subTestName, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.url, nil)
			w := httptest.NewRecorder()
			a.proxy.ServeHTTP(w, r)
			require.Equal(t, tc.statusCode, w.Code)
			require.Contains(t, w.Body.String(), tc.body)

		})
	}
}
