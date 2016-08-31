// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package httpmitm

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

func getBody(s string) (body []string) {
	pastHeaders := false
	for _, part := range strings.Split(s, "\r\n") {
		if pastHeaders {
			body = append(body, part)
		} else if part == "" {
			pastHeaders = true
		}
	}
	return
}

func Example() {
	// Setup test server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, Client!"))
	}))
	defer ts.Close()

	// Setup instrumented client.
	type record struct {
		o Origin
		d string
	}
	var records []*record
	client := http.Client{
		Transport: &Transport{
			Callback: func(o Origin, data []byte, err error) {
				records = append(records, &record{o, string(data)})
			},
		},
	}

	_, err := client.Post(ts.URL, "test", bytes.NewBufferString("Hail, Server!"))
	if err != nil {
		return
	}

	// There should be two records: request and response.
	for idx, r := range records {
		fmt.Printf("%d) %s\n", idx, r.o)
		for _, line := range getBody(r.d) {
			fmt.Println(line)
		}
	}

	// Output:
	// 0) Request
	// Hail, Server!
	// 1) Response
	// Hello, Client!
}
