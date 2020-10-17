package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/web"
)

func TestAddJSONRoute_ValidRoute_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(router *mux.Router, jsonRoute, expectedResponse string, expectedIncreasedCounter metrics2.Counter) {
		// Mock HTTP request and response.
		req, err := http.NewRequest("GET", jsonRoute, nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		// Call code under test, keeping track of the counter value.
		counterBefore := expectedIncreasedCounter.Get()
		router.ServeHTTP(rr, req)
		counterAfter := expectedIncreasedCounter.Get()

		require.Equal(t, http.StatusOK, rr.Code)
		require.Equal(t, expectedResponse, rr.Body.String())
		require.Equal(t, counterBefore+1, counterAfter)
	}

	fakeHandler := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(body))
			require.NoError(t, err)
		}
	}

	// Set up some routes to test against.
	router := mux.NewRouter()
	addJSONRoute(router, "", "/json/foo", fakeHandler("hello from /foo unversioned"))
	addJSONRoute(router, "", "/json/v1/foo", fakeHandler("hello from /foo v1"))
	addJSONRoute(router, "", "/json/v2/foo", fakeHandler("hello from /foo v2"))
	addJSONRoute(router, "", "/json/bar", fakeHandler("hello from /bar unversioned"))
	addJSONRoute(router, "", "/json/v1/bar", fakeHandler("hello from /bar v1"))
	addJSONRoute(router, "", "/json/v10/foo/{bar}/{baz}", fakeHandler("hello from /foo/{bar}/{baz} v10"))
	prefixedRouter := router.NewRoute().PathPrefix("/json").Subrouter()
	addJSONRoute(prefixedRouter, "/json", "/json/qux", fakeHandler("hello from /qux unversioned"))
	addJSONRoute(prefixedRouter, "/json", "/json/v1/qux", fakeHandler("hello from /qux v1"))

	counterFor := func(route, version string) metrics2.Counter {
		return metrics2.GetCounter(web.RPCCallCounterMetric, map[string]string{
			"route":   route,
			"version": version,
		})
	}

	// Test against routes from the root router.
	test(router, "/json/foo", "hello from /foo unversioned", counterFor("/foo", "v0"))
	test(router, "/json/v1/foo", "hello from /foo v1", counterFor("/foo", "v1"))
	test(router, "/json/v2/foo", "hello from /foo v2", counterFor("/foo", "v2"))
	test(router, "/json/bar", "hello from /bar unversioned", counterFor("/bar", "v0"))
	test(router, "/json/v1/bar", "hello from /bar v1", counterFor("/bar", "v1"))
	test(router, "/json/v10/foo/hello/world", "hello from /foo/{bar}/{baz} v10", counterFor("/foo/{bar}/{baz}", "v10"))

	// Test against routes from the prefixed router.
	test(router, "/json/qux", "hello from /qux unversioned", counterFor("/qux", "v0"))
	test(router, "/json/v1/qux", "hello from /qux v1", counterFor("/qux", "v1"))
}

func TestAddJSONRoute_InvalidRoute_Panics(t *testing.T) {
	unittest.SmallTest(t)

	test := func(routerPathPrefix, jsonRoute, expectedError string) {
		require.PanicsWithValue(t, expectedError, func() {
			router := mux.NewRouter()
			addJSONRoute(router, routerPathPrefix, jsonRoute, func(w http.ResponseWriter, r *http.Request) {})
		}, jsonRoute)
	}

	// Route is not prefixed with the router's path prefix.
	test("/json/alpha", "/json/beta/foo", `Prefix "/json/alpha" not found in JSON RPC route: /json/beta/foo`)

	// Interesting edge cases.
	test("", "/json", "Unrecognized JSON RPC route format: /json")
	test("", "/json/", "Unrecognized JSON RPC route format: /json/")
	test("", "/json/v0/myrpc", "JSON RPC version cannot be 0: /json/v0/myrpc")

	// Other invalid routes.
	test("", "", "Unrecognized JSON RPC route format: ")
	test("", "/", "Unrecognized JSON RPC route format: /")
	test("", "/foo", "Unrecognized JSON RPC route format: /foo")
	test("", "/foo/", "Unrecognized JSON RPC route format: /foo/")
	test("", "/foo/bar", "Unrecognized JSON RPC route format: /foo/bar")
}
