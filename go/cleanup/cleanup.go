package cleanup

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	cancel context.CancelFunc
	ctx    context.Context
	wg     sync.WaitGroup
)

// Initialize the package.
func init() {
	resetContext()
}

// Reset the context. This is in a non-init function for testing purposes.
func resetContext() {
	// The below should be unnecessary but makes "go vet" happy.
	newContext, newCancel := context.WithCancel(context.Background())
	ctx = newContext
	cancel = newCancel
}

// Repeat runs the tick function immediately and on the given timer. When
// Cancel() is called, the optional cleanup function is run after waiting for
// the tick function to finish.
func Repeat(tickFrequency time.Duration, tick, cleanup func()) {
	wg.Add(1)
	go func() {
		// Returns after gContext is canceled AND tick is finished.
		util.RepeatCtx(tickFrequency, ctx, tick)
		if cleanup != nil {
			cleanup()
		}
		wg.Done()
	}()
}

// Cleanup cancels all tick functions registered via Repeat(), then waits for
// them to fully stop running and for their cleanup functions to run.
func Cleanup() {
	sklog.Warningf("Shutdown request received")
	cancel()
	wg.Wait()
	sklog.Warningf("Finished clean shutdown procedure.")
}
