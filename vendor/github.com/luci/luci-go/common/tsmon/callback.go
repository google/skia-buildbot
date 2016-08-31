// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/tsmon/types"
)

// Callback is a function that is run at metric collection time to set the
// values of one or more metrics.  A callback can be registered with
// RegisterCallback.
type Callback func(ctx context.Context)

// A GlobalCallback is a Callback with the list of metrics it affects, so those
// metrics can be reset after they are flushed.
type GlobalCallback struct {
	Callback
	Metrics []types.Metric
}

// RegisterCallback registers a callback function that will be run at metric
// collection time to set the values of one or more metrics.  RegisterCallback
// should be called from an init() function in your module.
func RegisterCallback(f Callback) {
	RegisterCallbackIn(context.Background(), f)
}

// RegisterCallbackIn is like RegisterCallback but registers in a given context.
func RegisterCallbackIn(c context.Context, f Callback) {
	state := GetState(c)
	state.CallbacksMutex.Lock()
	defer state.CallbacksMutex.Unlock()

	state.Callbacks = append(state.Callbacks, f)
}

// RegisterGlobalCallback registers a callback function that will be run once
// per minute on *one* instance of your application. It is supported primarily
// on GAE.
//
// You must specify the list of metrics that your callback affects - these
// metrics will be reset after flushing to ensure they are not sent by multiple
// instances.
//
// RegisterGlobalCallback should be called from an init() function in your
// module.
func RegisterGlobalCallback(f Callback, metrics ...types.Metric) {
	RegisterGlobalCallbackIn(context.Background(), f, metrics...)
}

// RegisterGlobalCallbackIn is like RegisterGlobalCallback but registers in a
// given context.
func RegisterGlobalCallbackIn(c context.Context, f Callback, metrics ...types.Metric) {
	if len(metrics) == 0 {
		panic("RegisterGlobalCallback called without any metrics")
	}

	state := GetState(c)
	state.CallbacksMutex.Lock()
	defer state.CallbacksMutex.Unlock()

	state.GlobalCallbacks = append(state.GlobalCallbacks, GlobalCallback{f, metrics})
}
