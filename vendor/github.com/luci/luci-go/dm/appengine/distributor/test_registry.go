// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distributor

import (
	"fmt"

	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

type testRegistry struct {
	finishExecutionImpl FinishExecutionFn
	data                map[string]D
}

var _ Registry = (*testRegistry)(nil)

// NewTestingRegistry returns a new testing registry.
//
// The mocks dictionary maps from cfgName to a mock implementation of the
// distributor.
func NewTestingRegistry(mocks map[string]D, fFn FinishExecutionFn) Registry {
	return &testRegistry{fFn, mocks}
}

func (t *testRegistry) FinishExecution(c context.Context, eid *dm.Execution_ID, rslt *dm.Result) ([]tumble.Mutation, error) {
	return t.finishExecutionImpl(c, eid, rslt)
}

func (t *testRegistry) MakeDistributor(_ context.Context, cfgName string) (D, string, error) {
	ret, ok := t.data[cfgName]
	if !ok {
		return nil, "", fmt.Errorf("unknown distributor configuration: %q", cfgName)
	}
	return ret, "testing", nil
}
