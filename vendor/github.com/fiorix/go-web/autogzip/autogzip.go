// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// autogzip provides on-the-fly gzip encoding for http servers. It also has
// a client that decodes automatically when necessary (GetPage à-la Twisted).
package autogzip

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type ResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w ResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Handle provides on-the-fly gzip encoding for other handlers.
//
// Usage:
//
//	func DL1Handler(w http.ResponseWriter, req *http.Request) {
//		fmt.Fprintln(w, "foobar")
//	}
//
//	func DL2Handler(w http.ResponseWriter, req *http.Request) {
//		fmt.Fprintln(w, "zzz")
//	}
//
//	func main() {
//		http.HandleFunc("/download1", DL1Handler)
//		http.HandleFunc("/download2", DL2Handler)
//		http.ListenAndServe(":8080", autogzip.Handle(http.DefaultServeMux))
//	}
func Handle(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		h.ServeHTTP(ResponseWriter{Writer: gz, ResponseWriter: w}, r)
	}
}

// HandleFunc provides on-the-fly gzip encoding for other handler functions.
//
// Usage:
//
//	func IndexHandler(w http.ResponseWriter, req *http.Request) {
//		fmt.Fprintln(w, "Hello, world")
//	}
//
//	func DL1Handler(w http.ResponseWriter, req *http.Request) {
//		fmt.Fprintln(w, "foobar")
//	}
//
//	func main() {
//		http.HandleFunc("/", IndexHandler)
//		http.HandleFunc("/download1", autogzip.HandleFunc(DL1Handler))
//		http.ListenAndServe(":8080", nil)
//	}
func HandleFunc(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			fn(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		fn(ResponseWriter{Writer: gz, ResponseWriter: w}, r)
	}
}
