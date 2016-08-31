// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package router_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/router"
)

func Logger(c *router.Context, next router.Handler) {
	fmt.Println(c.Request.URL)
	next(c)
}

func AuthCheck(c *router.Context, next router.Handler) {
	var authenticated bool
	if !authenticated {
		c.Writer.WriteHeader(http.StatusUnauthorized)
		c.Writer.Write([]byte("Authentication failed"))
		return
	}
	next(c)
}

func GenerateSecret(c *router.Context, next router.Handler) {
	c.Context = context.WithValue(c.Context, "secret", rand.Int())
	next(c)
}

func makeRequest(client *http.Client, url string) string {
	res, err := client.Get(url + "/")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	p, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if len(p) == 0 {
		return fmt.Sprintf("%d", res.StatusCode)
	}
	return fmt.Sprintf("%d %s", res.StatusCode, p)
}

func makeRequests(url string) {
	c := &http.Client{}
	fmt.Println(makeRequest(c, url+"/hello"))
	fmt.Println(makeRequest(c, url+"/hello/darknessmyoldfriend"))
	fmt.Println(makeRequest(c, url+"/authenticated/secret"))
}

// Example_createServer demonstrates creating an HTTP server using the router
// package.
func Example_createServer() {
	r := router.New()
	r.Use(router.NewMiddlewareChain(Logger))
	r.GET("/hello", router.MiddlewareChain{}, func(c *router.Context) {
		fmt.Fprintf(c.Writer, "Hello")
	})
	r.GET("/hello/:name", router.MiddlewareChain{}, func(c *router.Context) {
		fmt.Fprintf(c.Writer, "Hello %s", c.Params.ByName("name"))
	})

	auth := r.Subrouter("authenticated")
	auth.Use(router.NewMiddlewareChain(AuthCheck))
	auth.GET("/secret", router.NewMiddlewareChain(GenerateSecret), func(c *router.Context) {
		fmt.Fprintf(c.Writer, "secret: %d", c.Context.Value("secret"))
	})

	server := httptest.NewServer(r)
	defer server.Close()

	makeRequests(server.URL)

	// Output:
	// /hello
	// 200 Hello
	// /hello/darknessmyoldfriend
	// 200 Hello darknessmyoldfriend
	// /authenticated/secret
	// 401 Authentication failed
}
