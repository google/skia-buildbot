// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cancelcond

import (
	"sync"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestTimeoutCond(t *testing.T) {
	t.Parallel()

	Convey(`A timeout Cond bound to a lock`, t, func() {
		c, cancelFunc := context.WithCancel(context.Background())
		l := sync.Mutex{}
		cc := New(&l)

		Convey(`Will cancel immediately if cancelled before the Wait.`, func() {
			l.Lock()
			defer l.Unlock()

			cancelFunc()
			So(cc.Wait(c), ShouldEqual, context.Canceled)
		})

		Convey(`Will unlock if cancelled.`, func() {
			l.Lock()
			defer l.Unlock()

			go func() {
				// Reclaim the lock. This ensures that "Wait()" has yielded the lock
				// prior to signalling.
				l.Lock()
				defer l.Unlock()

				cancelFunc()
			}()

			So(cc.Wait(c), ShouldEqual, context.Canceled)
		})

		Convey(`Will behave normally if not cancelled.`, func() {
			l.Lock()
			defer l.Unlock()

			go func() {
				// Reclaim the lock. This ensures that "Wait()" has yielded the lock
				// prior to signalling.
				l.Lock()
				defer l.Unlock()
				cc.Signal()
			}()

			So(cc.Wait(c), ShouldBeNil)
		})
	})
}
