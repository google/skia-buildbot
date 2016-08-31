// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"net/http"

	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"golang.org/x/net/context"
)

type testHTTPError int

func (e testHTTPError) Error() string {
	return http.StatusText(int(e))
}

func httpError(s int) error {
	return testHTTPError(s)
}

// testLogsServiceBase is an implementation of logdog.LogsServer that panics
// for each method. It is designed to be embedded in other testing instances.
type testLogsServiceBase struct{}

var _ logdog.LogsServer = (*testLogsServiceBase)(nil)

func (s *testLogsServiceBase) Get(c context.Context, req *logdog.GetRequest) (*logdog.GetResponse, error) {
	panic("not implemented")
}

func (s *testLogsServiceBase) Tail(c context.Context, req *logdog.TailRequest) (*logdog.GetResponse, error) {
	panic("not implemented")
}

func (s *testLogsServiceBase) Query(c context.Context, req *logdog.QueryRequest) (*logdog.QueryResponse, error) {
	panic("not implemented")
}

func (s *testLogsServiceBase) List(c context.Context, req *logdog.ListRequest) (*logdog.ListResponse, error) {
	panic("not implemented")
}
