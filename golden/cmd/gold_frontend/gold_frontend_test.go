package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/web"
)

func TestAddJSONRoute_ValidRoute_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(router *mux.Router, jsonRoute, expectedResponse string, callCountMetricExpectedToIncrease metrics2.Counter) {
		// Mock HTTP request and response.
		req, err := http.NewRequest("GET", jsonRoute, nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		// Call code under test, keeping track of the metric value.
		countBefore := callCountMetricExpectedToIncrease.Get()
		router.ServeHTTP(rr, req)
		countAfter := callCountMetricExpectedToIncrease.Get()

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, expectedResponse, rr.Body.String())
		assert.Equal(t, int64(0), countBefore)
		assert.Equal(t, int64(1), countAfter)
	}

	// We will use the string written by our fake handler functions to assert that each route added
	// with addJSONRoute is associated with the right handler function.
	fakeHandler := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write([]byte(body))
			require.NoError(t, err)
		}
	}

	// Set up some routes to test against.
	router := mux.NewRouter()
	addJSONRoute("/json/foo", fakeHandler("hello from /foo unversioned"), router, "")
	addJSONRoute("/json/v1/foo", fakeHandler("hello from /foo v1"), router, "")
	addJSONRoute("/json/v2/foo", fakeHandler("hello from /foo v2"), router, "")
	addJSONRoute("/json/bar", fakeHandler("hello from /bar unversioned"), router, "")
	addJSONRoute("/json/v1/bar", fakeHandler("hello from /bar v1"), router, "")
	addJSONRoute("/json/v10/foo/{bar}/{baz}", fakeHandler("hello from /foo/{bar}/{baz} v10"), router, "")
	prefixedRouter := router.NewRoute().PathPrefix("/json").Subrouter()
	addJSONRoute("/json/qux", fakeHandler("hello from /qux unversioned"), prefixedRouter, "/json")
	addJSONRoute("/json/v1/qux", fakeHandler("hello from /qux v1"), prefixedRouter, "/json")

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
			addJSONRoute(jsonRoute, func(w http.ResponseWriter, r *http.Request) {}, router, routerPathPrefix)
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
