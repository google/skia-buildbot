// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.
package store

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/tsmon/store/storetest"
	"github.com/luci/luci-go/common/tsmon/target"
	"golang.org/x/net/context"
)

func TestInMemory(t *testing.T) {
	ctx := context.Background()
	storetest.RunStoreImplementationTests(t, ctx, storetest.TestOptions{
		Factory: func() storetest.Store {
			return NewInMemory(&target.Task{ServiceName: proto.String("default target")})
		},
		RegistrationFinished: func(storetest.Store) {},
		GetNumRegisteredMetrics: func(s storetest.Store) int {
			return len(s.(*inMemoryStore).data)
		},
	})
}
