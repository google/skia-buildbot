package common

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
	sigIntHandler  *signalHandler
	sigTermHandler *signalHandler
)

func init() {
	sigIntHandler = newSignalHandler(syscall.SIGINT)
	sigTermHandler = newSignalHandler(syscall.SIGTERM)
}

// OnSigInt adds a handler func which runs if SIGINT is received.
func OnSigInt(fn func()) {
	sigIntHandler.addCallback(fn)
}

// OnSigTerm adds a handler func which runs if SIGTERM is received.
func OnSigTerm(fn func()) {
	sigTermHandler.addCallback(fn)
}

// Set up signal handlers. This should be called by all variants of
// common.Init().
func setupSignalHandlers() {
	sigIntHandler.enable()
	sigTermHandler.enable()
}

// signalHandler is a struct which manages multiple callback functions for a
// given signal.
type signalHandler struct {
	callbacks []func()
	mtx       sync.Mutex
	signal    syscall.Signal
}

// newSignalHandler creates and returns a signalHandler for the given signal.
func newSignalHandler(sig syscall.Signal) *signalHandler {
	return &signalHandler{
		callbacks: []func(){},
		signal:    sig,
	}
}

// Handle enables the signalHandler.
func (sh *signalHandler) enable() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, sh.signal)
	go func() {
		_ = <-c
		sh.mtx.Lock()
		defer sh.mtx.Unlock()
		sklog.Errorf("Caught %s", sh.signal)
		for _, fn := range sh.callbacks {
			func() {
				defer func() {
					if r := recover(); r != nil {
						sklog.Errorf("Panic: %s", r)
					}
				}()
				fn()
			}()
		}
		sklog.Flush()
		// Special code for interrupt, according to
		// http://tldp.org/LDP/abs/html/exitcodes.html
		os.Exit(130)
	}()
}

// addCallback adds a callback function to run when the given signal is received.
func (sh *signalHandler) addCallback(fn func()) {
	sh.mtx.Lock()
	defer sh.mtx.Unlock()
	sh.callbacks = append(sh.callbacks, fn)
}
