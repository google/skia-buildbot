// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ctxcmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/luci/luci-go/common/clock"
	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	envHelperTest = "GO_CTXCMD_HELPER_TEST=1"

	// waitForeverReady is text emitted by process when it has installed its
	// signal handler and begun waiting forever.
	waitForeverReady = "Waiting forever..."
)

func isHelperTest() bool {
	for _, v := range os.Environ() {
		if v == envHelperTest {
			return true
		}
	}
	return false
}

func helperCommand(t *testing.T, name string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], fmt.Sprintf("-test.run=%s", name))
	cmd.Env = []string{envHelperTest}
	return cmd
}

func TestCtxCmd(t *testing.T) {
	t.Parallel()

	Convey(`A cancellable context`, t, func() {
		c, cancelFunc := context.WithCancel(context.Background())

		Convey(`When running a bogus process`, func() {
			cc := CtxCmd{
				Cmd: exec.Command("##fake-does-not-exist##"),
			}

			Convey(`If the Context is already cancelled, the process will not run.`, func() {
				cancelFunc()
				So(cc.Start(c), ShouldEqual, context.Canceled)
			})
		})

		Convey(`When running a process that exits immediately`, func() {
			cc := CtxCmd{
				Cmd:  helperCommand(t, "TestExitImmediately"),
				test: &testCallbacks{},
			}

			Convey(`The process runs and exits successfully.`, func() {
				So(cc.Run(c), ShouldBeNil)
			})

			Convey(`Cancelling after process exit returns process' exit value.`, func() {
				cc.test.finishedCB = func() {
					cancelFunc()
				}

				// Make sure that we got a process return value.
				So(cc.Run(c), ShouldBeNil)
				So(cc.ProcessError, ShouldBeNil)
			})
		})

		Convey(`When running a process that runs forever`, func() {
			cc := CtxCmd{
				Cmd:  helperCommand(t, "TestWaitForever"),
				test: &testCallbacks{},
			}

			Convey(`Cancelling the process causes it to exit with non-zero return code.`, func() {
				So(cc.Start(c), ShouldBeNil)
				cancelFunc()

				So(cc.Wait(), ShouldEqual, context.Canceled)

				So(cc.ProcessError, ShouldHaveSameTypeAs, (*exec.ExitError)(nil))

				ec, ok := ExitCode(cc.ProcessError)
				So(ok, ShouldBeTrue)
				So(ec, ShouldNotEqual, 0)
			})

			Convey(`Interrupting the process causes it to exit with return code 5 (see main()).`, func() {
				if runtime.GOOS == "windows" {
					// This test does not work on Windows, as processes cannot send
					// signals (e.g., os.Interrupt) to other processes.
					return
				}

				// Repeatedly send SIGINT to the process. This is something introduced
				// in Go 1.6, where SIGINT is successfully sent to the process (Signal
				// returns nil), but the process doesn't receive it. We will, therefore,
				// cancel the process repeatedly once the initial signal has been sent.
				//
				// This is handled by test code instead of main CtxCmd monitor because
				// it is actually an issue with the test child process' runtime, not
				// CtxCmd itself.
				//
				// Possibly: https://github.com/golang/go/issues/14571
				finishedC := make(chan struct{})
				cc.test.finishedCB = func() {
					close(finishedC)
				}
				cc.test.canceledCB = func() {
					go func() {
						for {
							select {
							case <-finishedC:
								return
							default:
								cc.sendCancelSignal()
								clock.Sleep(context.Background(), 10*time.Millisecond)
							}
						}
					}()
				}

				// Create a pipe to ensure that the process has started.
				rc, err := cc.Cmd.StdoutPipe()
				So(err, ShouldBeNil)
				defer rc.Close()

				cc.CancelSignal = os.Interrupt
				So(cc.Start(c), ShouldBeNil)

				// Read "Waiting forever..."
				buf := make([]byte, len(waitForeverReady))
				_, err = rc.Read(buf)
				So(err, ShouldBeNil)
				So(string(buf), ShouldEqual, waitForeverReady)

				// Process is ready, go ahead and cancel.
				cancelFunc()

				So(cc.Wait(), ShouldEqual, context.Canceled)

				So(cc.ProcessError, ShouldHaveSameTypeAs, (*exec.ExitError)(nil))

				ec, ok := ExitCode(cc.ProcessError)
				So(ok, ShouldBeTrue)
				So(ec, ShouldEqual, 5)
			})
		})

		Convey(`When running a process with a non-zero exit code`, func() {
			cc := CtxCmd{
				Cmd: helperCommand(t, "TestExitWithError"),
			}

			Convey(`Run returns an error.`, func() {
				err := cc.Run(c)
				So(err, ShouldNotBeNil)
				So(cc.ProcessError, ShouldHaveSameTypeAs, (*exec.ExitError)(nil))

				ec, ok := ExitCode(err)
				So(ok, ShouldBeTrue)
				So(ec, ShouldEqual, 42)
			})
		})
	})
}
