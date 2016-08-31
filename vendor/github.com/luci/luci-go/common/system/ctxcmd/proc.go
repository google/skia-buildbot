// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ctxcmd

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/net/context"
)

// Error is the type of error returned by Wait().
type cmdResult struct {
	// err is the acutal error to return from Wait.
	err error
	// state is the process' returned state.
	state *os.ProcessState
	// procErr is the process' returned error. Depending on whether the process
	// was cancelled, this may equal err.
	procErr error
	// cancelErr is the error (if any) encountered when canceling the process.
	// This should be nil during standard operations.
	cancelErr error
}

// CtxCmd is a wrapper around an exec.Cmd that responds to a Context's
// cancellation by terminating the process.
//
// A Cmd managed by CtxCmd should not have its Run, Start, or Wait methods
// used.
type CtxCmd struct {
	*exec.Cmd

	// ProcessError is the error returned by the process. It is populated when
	// Run() or Wait() exits.
	//
	// If the process executes successfully and returns a zero exit code, this
	// will be nil.
	//
	// This may differ from the return value of Run() or Wait() if the process
	// was cancelled.
	ProcessError error

	// CancelError is the error that was encountered when canceling the process.
	// This should be nil during standard operation, and does not need to be
	// checked.
	CancelError error

	// CancelSignal, if not nil, is the signal that is sent to the process to
	// terminate it when it is cancelled. If nil, the os.Kill signal will be sent.
	CancelSignal os.Signal

	// beginWaitC is closed at the beginning of Wait() to indicate that the
	// process' Wait() should be called. We do this to time our Wait() with our
	// caller's Wait() invocation so we don't interrupt their process pipes.
	beginWaitC chan struct{}
	waitC      chan *cmdResult

	// test is configured during testing to instrument CtxCmd internals. It is
	// nil during non-testing operation.
	test *testCallbacks
}

type testCallbacks struct {
	// finishedCB, if not nil, called when the process has finished.
	finishedCB func()

	// canceledCB, if not nil, called when the process is canceled.
	canceledCB func()
}

// Run starts the process, blocking until it has exited. It is the analogue to
// the underlying Cmd.Run().
//
// If the supplied Context is cancelled (deadline, cancel, etc.), the process
// will be killed prematurely.
func (cc *CtxCmd) Run(c context.Context) error {
	if err := cc.Start(c); err != nil {
		return err
	}
	return cc.Wait()
}

// Start starts the process and returns. It is the analogue to the underlying
// Cmd.Start().
//
// The process should then be reaped via Wait().
//
// If the supplied Context is cancelled (deadline, cancel, etc.), the process
// will be killed prematurely.
func (cc *CtxCmd) Start(c context.Context) error {
	// Check if the Context has been cancelled before wasting our time executing.
	select {
	case <-c.Done():
		return c.Err()

	default:
		break
	}

	if err := cc.Cmd.Start(); err != nil {
		return err
	}

	// Begin Waiting on the process. This is asynchronous and will ultimately
	// feed back into Wait().
	beginWaitC := make(chan struct{})
	finishedC := make(chan *cmdResult, 1)
	go func() {
		var r cmdResult
		defer func() {
			finishedC <- &r
		}()

		<-beginWaitC
		r.procErr = cc.Cmd.Wait()
		r.state = cc.ProcessState
	}()

	// Start a monitor to wait on Context cancel or process exit.
	waitC := make(chan *cmdResult)
	go func() {
		defer close(finishedC)

		// Write the process result to "waitC".
		var r *cmdResult
		defer func() {
			defer close(waitC)

			// (Testing) invoke our finished callback, if one is configured.
			if cc.test != nil && cc.test.finishedCB != nil {
				cc.test.finishedCB()
			}

			waitC <- r
		}()

		select {
		case r = <-finishedC:
			r.err = r.procErr

		case <-c.Done():
			// Make sure the process didn't already finish.
			select {
			case r = <-finishedC:
				r.err = r.procErr
				return

			default:
				// Go ahead and kill/cancel.
				break
			}

			// Cancel the process (signal/kill).
			cancelErr := cc.cancel()
			r = <-finishedC

			r.err = c.Err()
			r.cancelErr = cancelErr
		}
	}()

	cc.beginWaitC = beginWaitC
	cc.waitC = waitC
	return nil
}

// Wait is the cancellable analogue of Cmd's Wait(). It blocks until the process
// exits.
//
// If the Context was cancelled, this will still block until the process exits.
func (cc *CtxCmd) Wait() error {
	// Detect if we've already performed a Wait.
	select {
	case <-cc.beginWaitC:
		return errors.New("wait has already been called for this process")
	default:
		break
	}

	// Signal our internal Wait monitor goroutine to Wait and cleanup.
	close(cc.beginWaitC)

	// Record our process' Wait result.
	r := <-cc.waitC
	cc.ProcessState, cc.ProcessError, cc.CancelError = r.state, r.procErr, r.cancelErr

	// If our process ran, but exited with a non-zero error code, return an error
	// (follows contract of exec.Cmd's Wait)
	if r.err != nil {
		return r.err
	}
	return r.procErr
}

func (cc *CtxCmd) cancel() (err error) {
	err = cc.sendCancelSignal()

	// (Testing) invoke our canceled callback, if one is configured.
	if cc.test != nil && cc.test.canceledCB != nil {
		cc.test.canceledCB()
	}
	return
}

func (cc *CtxCmd) sendCancelSignal() error {
	switch {
	case cc.CancelSignal != nil:
		return cc.Signal(cc.CancelSignal)
	default:
		return cc.Kill()
	}
}

// Kill sends a kill signal to the underlying process. This only works if the
// Process is currently running.
func (cc *CtxCmd) Kill() error {
	return cc.Cmd.Process.Kill()
}

// Signal sends a signal to the underlying process. This only works if the
// Process is currently running.
func (cc *CtxCmd) Signal(sig os.Signal) error {
	return cc.Cmd.Process.Signal(sig)
}

// ExitCode returns the process exit code given an error. If no exit code is
// present, 0 will be returned.
func ExitCode(err error) (int, bool) {
	if err == nil {
		return 0, true
	}

	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Sys().(syscall.WaitStatus).ExitStatus(), true
	}
	return 0, false
}
