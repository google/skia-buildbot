// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package httpxtra

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"
)

// LoggerFunc are functions called by httpxtra.Handler at the end of each request.
type LoggerFunc func(r *http.Request, created time.Time, status, bytes int)

type LogWriter struct {
	ResponseWriter http.ResponseWriter
	Bytes          int
	Status         int
}

func (lw *LogWriter) Header() http.Header {
	return lw.ResponseWriter.Header()
}

func (lw *LogWriter) Write(b []byte) (int, error) {
	if lw.Status == 0 {
		lw.Status = http.StatusOK
	}
	n, err := lw.ResponseWriter.Write(b)
	lw.Bytes += n
	return n, err
}

func (lw *LogWriter) WriteHeader(s int) {
	lw.ResponseWriter.WriteHeader(s)
	lw.Status = s
}

func (lw *LogWriter) Flush() {
	lw.ResponseWriter.(http.Flusher).Flush()
}

func (lw *LogWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if lw.Status == 0 {
		lw.Status = http.StatusOK
	}
	// TODO: Check. Does it break if the server don't support hijacking?
	return lw.ResponseWriter.(http.Hijacker).Hijack()
}

func (lw *LogWriter) CloseNotify() <-chan bool {
	return lw.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

// ApacheCommonLog returns an Apache Common access log string.
func ApacheCommonLog(r *http.Request, created time.Time, status, bytes int) string {
	u := "-"
	if r.URL.User != nil {
		if name := r.URL.User.Username(); name != "" {
			u = name
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return fmt.Sprintf("%s - %s [%s] \"%s %s %s\" %d %d",
		ip,
		u,
		created.Format("02/Jan/2006:15:04:05 -0700"),
		r.Method,
		r.RequestURI,
		r.Proto,
		status,
		bytes,
	)
}
