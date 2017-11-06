package repeat_cleanup

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	cancelContext context.CancelFunc
	gContext      context.Context = context.Background()
	wg            sync.WaitGroup
)

// Init initializes the package.
func Init(ctx context.Context) context.Context {
	cancelable, cancelFn := context.WithCancel(ctx)
	gContext = cancelable
	cancelContext = cancelFn
	return gContext
}

// Repeat runs the tick function immediately and on the given timer. When
// Cancel() is called, the optional cleanup function is run after waiting for
// the tick function to finish.
func Repeat(tickFrequency time.Duration, tick, cleanup func()) {
	wg.Add(1)
	go func() {
		// Returns after gContext is canceled AND tick is finished.
		util.RepeatCtx(tickFrequency, gContext, tick)
		if cleanup != nil {
			cleanup()
		}
		wg.Done()
	}()
}

// Cancel cancels all tick functions registered via Repeat(), then waits for
// them to fully stop running and for their cleanup functions to run.
func Cancel() {
	sklog.Warningf("Shutdown request received")
	cancelContext()
	wg.Wait()
	sklog.Warningf("Finished clean shutdown procedure.")
}
