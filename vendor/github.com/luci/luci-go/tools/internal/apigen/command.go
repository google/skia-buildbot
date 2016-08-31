// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package apigen

import (
	"os"
	"os/exec"
	"time"

	"github.com/luci/luci-go/common/clock"
	log "github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

const shutdownWait = 20 * time.Second

// killableCommand is an extension of exec.Cmd that can be easily killed.
type killableCommand struct {
	*exec.Cmd
}

func (cmd *killableCommand) kill(c context.Context) {
	c = log.SetField(c, "cmd", cmd.Path)

	// First, try and nicely kill the process.
	log.Debugf(c, "Sending interrupt to process.")
	cmd.Process.Signal(os.Interrupt)

	processFinishedC := make(chan struct{})
	defer close(processFinishedC)
	go func() {
		total := time.Duration(0)
		t := clock.NewTimer(c)
		for total < shutdownWait {
			t.Reset(time.Second)
			select {
			case <-processFinishedC:
				return

			case <-t.GetC():
				// Signal the process again.
				total += time.Second
				log.Debugf(c, "Process not terminated; sending interrupt to process.")
				cmd.Process.Signal(os.Interrupt)
			}
		}

		log.Debugf(c, "Process did not gracefully exit; sending KILL.")
		cmd.Process.Kill()
	}()

	cmd.Wait()
	log.Debugf(c, "Process terminated.")
}
