package cleanup

import (
	"context"
	"sync"
	"syscall"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	atExitFns []func()
	atExitMtx sync.Mutex
	cancel    context.CancelFunc
	ctx       context.Context
	once      sync.Once
	wg        sync.WaitGroup
)

// Initialize the package.
func init() {
	intHandler = newHandler(syscall.SIGINT, syscall.SIGTERM)
	reset()
	onInterrupt(Cleanup)
}

// Reset the package. This is in a non-init function for testing purposes.
func reset() {
	// The below should be unnecessary but makes "go vet" happy.
	newContext, newCancel := context.WithCancel(context.Background())
	ctx = newContext
	cancel = newCancel
	once = sync.Once{}
}

// AtExit runs the given function before the program exits, either naturally,
// or when a signal is received. In order to run the function on normal exit,
// Cleanup() should be called at the end of main(). common.Defer() is the
// canonical way to do this.
func AtExit(fn func()) {
	atExitMtx.Lock()
	defer atExitMtx.Unlock()
	atExitFns = append(atExitFns, fn)
}

// Repeat runs the tick function immediately and on the given timer. When
// Cancel() is called, waits for any active tick() to finish (tick may or may
// not respect ctx.Done), and then the optional cleanup function is run.
func Repeat(tickFrequency time.Duration, tick func(context.Context), cleanup func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Returns after gContext is canceled AND tick is finished.
		util.RepeatCtx(tickFrequency, ctx, tick)
		if cleanup != nil {
			cleanup()
		}
	}()
}

// Cleanup cancels all tick functions registered via Repeat(), then waits for
// them to fully stop running and for their cleanup functions to run. Cleanup()
// runs automatically when SIGINT or SIGTERM is received. If your program runs
// interactively or is expected to exit normally under other circumstances, you
// should make sure Cleanup() is called at the end of main(). common.Defer() is
// the canonical way to do this.
func Cleanup() {
	once.Do(func() {
		sklog.Warningf("Running clean shutdown procedures.")
		cancel()
		for _, fn := range atExitFns {
			func() {
				defer func() {
					if r := recover(); r != nil {
						sklog.Errorf("Panic during AtExit func: %s", r)
					}
				}()
				fn()
			}()
		}
		wg.Wait()
		sklog.Warningf("Finished clean shutdown procedures.")
	})
}
