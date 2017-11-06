package quitquitquit

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	cancelContext context.CancelFunc
	gContext      context.Context = context.Background()

	entries []*entry
	mtx     sync.Mutex

	srv http.Server
)

// entry represents one call to Repeat().
type entry struct {
	shutdownFinished bool
	mtx              sync.Mutex
}

// done() returns true iff the given entry has been successfully canceled and
// cleaned up.
func (e *entry) done() bool {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.shutdownFinished
}

// Init adds a listener for a /quitquitquit endpoint which runs Cancel().
func Init(port string, ctx context.Context) context.Context {
	cancelable, cancelFn := context.WithCancel(ctx)
	gContext = cancelable
	cancelContext = cancelFn
	r := mux.NewRouter()
	r.HandleFunc("/quitquitquit", func(w http.ResponseWriter, r *http.Request) {
		sklog.Warningf("Shutdown request received")
		Cancel()
		sklog.Warningf("Finished clean shutdown procedure.")
	})
	srv.Addr = port
	srv.Handler = r
	go func() {
		util.LogErr(srv.ListenAndServe())
	}()
	return gContext
}

// Repeat runs the tick function immediately and on the given timer. When
// Cancel() is called or the /quitquitquit endpoint is hit, the optional cleanup
// function is run after waiting for the tick function to finish.
func Repeat(tickFrequency time.Duration, tick, cleanup func()) {
	e := &entry{}
	mtx.Lock()
	defer mtx.Unlock()
	entries = append(entries, e)
	go func() {
		// Returns after gContext is canceled AND tick is finished.
		util.RepeatCtx(tickFrequency, gContext, tick)
		if cleanup != nil {
			cleanup()
		}
		e.mtx.Lock()
		defer e.mtx.Unlock()
		e.shutdownFinished = true
	}()
}

// Cancel cancels all tick functions registered via Repeat(), then waits for
// them to fully stop running and for their cleanup functions to run.
func Cancel() {
	cancelContext()
	for _, e := range entries {
		for !e.done() {
			time.Sleep(200 * time.Millisecond)
		}
	}
}
