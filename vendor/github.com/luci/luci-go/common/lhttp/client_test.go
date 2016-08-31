// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package lhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/retry"

	"github.com/maruel/ut"
)

func httpReqGen(method, url string, body []byte) RequestGen {
	return func() (*http.Request, error) {
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}
		return http.NewRequest("GET", url, bodyReader)
	}
}

func TestNewRequestGET(t *testing.T) {
	ctx := context.Background()

	// First call returns HTTP 500, second succeeds.
	serverCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		content, err := ioutil.ReadAll(r.Body)
		ut.ExpectEqual(t, nil, err)
		ut.ExpectEqual(t, []byte{}, content)
		if serverCalls == 1 {
			w.WriteHeader(500)
		} else {
			fmt.Fprintf(w, "Hello, client\n")
		}
	}))
	defer ts.Close()

	httpReq := httpReqGen("GET", ts.URL, nil)

	clientCalls := 0
	clientReq := NewRequest(ctx, http.DefaultClient, fast, httpReq, func(resp *http.Response) error {
		clientCalls++
		content, err := ioutil.ReadAll(resp.Body)
		ut.AssertEqual(t, nil, err)
		ut.AssertEqual(t, "Hello, client\n", string(content))
		ut.AssertEqual(t, nil, resp.Body.Close())
		return nil
	})

	status, err := clientReq()
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, 200, status)
	ut.AssertEqual(t, 2, serverCalls)
	ut.AssertEqual(t, 1, clientCalls)
}

func TestNewRequestPOST(t *testing.T) {
	ctx := context.Background()

	// First call returns HTTP 500, second succeeds.
	serverCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		content, err := ioutil.ReadAll(r.Body)
		ut.ExpectEqual(t, nil, err)
		// The same data is sent twice.
		ut.ExpectEqual(t, "foo bar", string(content))
		if serverCalls == 1 {
			w.WriteHeader(500)
		} else {
			fmt.Fprintf(w, "Hello, client\n")
		}
	}))
	defer ts.Close()

	httpReq := httpReqGen("POST", ts.URL, []byte("foo bar"))

	clientCalls := 0
	clientReq := NewRequest(ctx, http.DefaultClient, fast, httpReq, func(resp *http.Response) error {
		clientCalls++
		content, err := ioutil.ReadAll(resp.Body)
		ut.AssertEqual(t, nil, err)
		ut.AssertEqual(t, "Hello, client\n", string(content))
		ut.AssertEqual(t, nil, resp.Body.Close())
		return nil
	})

	status, err := clientReq()
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, 200, status)
	ut.AssertEqual(t, 2, serverCalls)
	ut.AssertEqual(t, 1, clientCalls)
}

func TestNewRequestGETFail(t *testing.T) {
	ctx := context.Background()

	serverCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.WriteHeader(500)
	}))
	defer ts.Close()

	httpReq := httpReqGen("GET", ts.URL, nil)

	clientReq := NewRequest(ctx, http.DefaultClient, fast, httpReq, func(resp *http.Response) error {
		t.Fail()
		return nil
	})

	status, err := clientReq()
	ut.AssertEqual(t, true, errors.IsTransient(err))
	ut.AssertEqual(t, "http request failed: Internal Server Error (HTTP 500)", err.Error())
	ut.AssertEqual(t, 500, status)
}

func TestGetJSON(t *testing.T) {
	ctx := context.Background()

	// First call returns HTTP 500, second succeeds.
	serverCalls := 0
	ts := httptest.NewServer(handlerJSON(t, func(body io.Reader) interface{} {
		serverCalls++
		content, err := ioutil.ReadAll(body)
		ut.ExpectEqual(t, nil, err)
		ut.ExpectEqual(t, []byte{}, content)
		if serverCalls == 1 {
			return nil
		}
		return map[string]string{"success": "yeah"}
	}))
	defer ts.Close()

	actual := map[string]string{}
	status, err := GetJSON(ctx, fast, http.DefaultClient, ts.URL, &actual)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, 200, status)
	ut.AssertEqual(t, map[string]string{"success": "yeah"}, actual)
	ut.AssertEqual(t, 2, serverCalls)
}

func TestGetJSONBadResult(t *testing.T) {
	ctx := context.Background()

	serverCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", jsonContentType)
		_, err := io.WriteString(w, "yo")
		ut.ExpectEqual(t, nil, err)
	}))
	defer ts.Close()

	actual := map[string]string{}
	status, err := GetJSON(ctx, fast, http.DefaultClient, ts.URL, &actual)
	ut.AssertEqual(t, true, errors.IsTransient(err))
	ut.AssertEqual(t, "bad response "+ts.URL+": invalid character 'y' looking for beginning of value", err.Error())
	ut.AssertEqual(t, 200, status)
	ut.AssertEqual(t, map[string]string{}, actual)
}

func TestGetJSONBadResultIgnore(t *testing.T) {
	ctx := context.Background()

	serverCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", jsonContentType)
		_, err := io.WriteString(w, "yo")
		ut.ExpectEqual(t, nil, err)
	}))
	defer ts.Close()

	status, err := GetJSON(ctx, fast, http.DefaultClient, ts.URL, nil)
	ut.AssertEqual(t, true, errors.IsTransient(err))
	ut.AssertEqual(t, "bad response "+ts.URL+": invalid character 'y' looking for beginning of value", err.Error())
	ut.AssertEqual(t, 200, status)
}

func TestGetJSONBadContentTypeIgnore(t *testing.T) {
	ctx := context.Background()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, "{}")
		ut.ExpectEqual(t, nil, err)
	}))
	defer ts.Close()

	status, err := GetJSON(ctx, fast, http.DefaultClient, ts.URL, nil)
	ut.AssertEqual(t, "unexpected Content-Type, expected \"application/json\", got \"text/plain; charset=utf-8\"", err.Error())
	ut.AssertEqual(t, 200, status)
}

func TestPostJSON(t *testing.T) {
	ctx := context.Background()

	// First call returns HTTP 500, second succeeds.
	serverCalls := 0
	ts := httptest.NewServer(handlerJSON(t, func(body io.Reader) interface{} {
		serverCalls++
		data := map[string]string{}
		ut.ExpectEqual(t, nil, json.NewDecoder(body).Decode(&data))
		ut.ExpectEqual(t, map[string]string{"in": "all"}, data)
		if serverCalls == 1 {
			return nil
		}
		return map[string]string{"success": "yeah"}
	}))
	defer ts.Close()

	in := map[string]string{"in": "all"}
	actual := map[string]string{}
	status, err := PostJSON(ctx, fast, http.DefaultClient, ts.URL, nil, in, &actual)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, 200, status)
	ut.AssertEqual(t, map[string]string{"success": "yeah"}, actual)
	ut.AssertEqual(t, 2, serverCalls)
}

func TestPostJSONwithHeaders(t *testing.T) {
	ctx := context.Background()

	serverCalls := 0
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			w.Header().Set("Content-Type", jsonContentType)
			ut.ExpectEqual(t, nil, json.NewEncoder(w).Encode(map[string]string{}))
			ut.ExpectEqual(t, r.Header.Get("key"), "value")
			serverCalls++
		}))
	defer ts.Close()

	status, err := PostJSON(ctx, fast, http.DefaultClient, ts.URL, map[string]string{"key": "value"}, nil, nil)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, 200, status)
	ut.AssertEqual(t, 1, serverCalls)
}

// Private details.

func fast() retry.Iterator {
	return &retry.Limited{Retries: 3}
}

type jsonAPI func(body io.Reader) interface{}

// handlerJSON converts a jsonAPI http handler to a proper http.Handler.
func handlerJSON(t *testing.T, handler jsonAPI) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//ut.ExpectEqual(t, jsonContentType, r.Header.Get("Content-Type"))
		defer r.Body.Close()
		out := handler(r.Body)
		if out == nil {
			w.WriteHeader(500)
		} else {
			w.Header().Set("Content-Type", jsonContentType)
			ut.ExpectEqual(t, nil, json.NewEncoder(w).Encode(out))
		}
	})
}
