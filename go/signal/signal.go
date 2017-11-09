package signal

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
	mtx       sync.Mutex
	signals   []os.Signal
}

// newHandler creates and returns a signalHandler for the given signal.
func newHandler(sigs ...os.Signal) *signalHandler {
	sh := &signalHandler{
		callbacks: []func(){},
		signals:   sigs,
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, sh.signals...)
	var once sync.Once
	go func() {
		sig := <-c
		once.Do(func() {
			sh.mtx.Lock()
			defer sh.mtx.Unlock()
			sklog.Errorf("Caught %s", sig)
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
	}()
	return sh
}

// addCallback adds a callback function to run when a given signal is received.
// Each callback will only run once, even if multiple signals are received.
func (sh *signalHandler) addCallback(fn func()) {
	sh.mtx.Lock()
	defer sh.mtx.Unlock()
	sh.callbacks = append(sh.callbacks, fn)
}

// OnInterrupt runs the given function when any of os.Interrupt, syscall.SIGINT,
// or syscall.SIGTERM is received. The function will only run once, even if more
// than one signal is received.
func OnInterrupt(fn func()) {
	if intHandler == nil {
		intHandler = newHandler(os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	}
	intHandler.addCallback(fn)
}
