package cleanup

/*
   Utilities for signal handing.
*/

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.skia.org/infra/go/sklog"
)

var (
	intHandler *signalHandler
)

// signalHandler is a struct which manages multiple callback functions for a
// set of signals.
type signalHandler struct {
	callbacks []func()
	chDisable chan bool
	chSignals chan os.Signal
	mtx       sync.Mutex
	signals   []os.Signal
}

// newHandler creates and returns a signalHandler for the given signal.
func newHandler(sigs ...os.Signal) *signalHandler {
	return &signalHandler{
		callbacks: []func(){},
		chDisable: make(chan bool, 1),
		chSignals: make(chan os.Signal, 1),
		signals:   sigs,
	}
}

// Disable signal handling for this signalHandler.
func (sh *signalHandler) disable() {
	signal.Reset(sh.signals...)
	sh.chDisable <- true
}

// Enable signal handling for this signalHandler.
func (sh *signalHandler) enable() {
	signal.Notify(sh.chSignals, sh.signals...)
	var once sync.Once
	go func() {
		select {
		case sig := <-sh.chSignals:
			once.Do(func() {
				sh.mtx.Lock()
				defer sh.mtx.Unlock()
				sklog.Warningf("Caught %s", sig)
				for _, fn := range sh.callbacks {
					func() {
						defer func() {
							if r := recover(); r != nil {
								sklog.Errorf("Panic during handler for signal %s: %s", sig, r)
							}
						}()
						fn()
					}()
				}
				sklog.Flush()

				// Exit with the correct code, according to:
				// http://tldp.org/LDP/abs/html/exitcodes.html
				//
				// Note: if not for this line, signalHandler could be
				// made public so that it could be used to handle any
				// signal, eg. SIGUSR1, for whatever reason. Since we
				// generally use HTTP endpoints for communication
				// between servers, we don't anticipate needing it, so
				// this is left here for simplicity under the assumption
				// that we only handle signals which should cause us to
				// exit.
				os.Exit(128 + int(sig.(syscall.Signal)))
			})
		case <-sh.chDisable:
			return
		}
	}()
}

// addCallback adds a callback function to run when a given signal is received.
// Each callback will only run once, even if multiple signals are received.
func (sh *signalHandler) addCallback(fn func()) {
	sh.mtx.Lock()
	defer sh.mtx.Unlock()
	sh.callbacks = append(sh.callbacks, fn)
}

// Enable signal handling for the cleanup package.
func Enable() {
	intHandler.enable()
}

// Disable signal handling for the cleanup package.
func Disable() {
	intHandler.disable()
}

// onInterrupt runs the given function when any of syscall.SIGINT or
// syscall.SIGTERM is received. The function will only run once, even if more
// than one signal is received.
func onInterrupt(fn func()) {
	intHandler.addCallback(fn)
}
