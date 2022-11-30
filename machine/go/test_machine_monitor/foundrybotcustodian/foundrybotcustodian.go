// Package foundrybotcustodian starts Foundry Bot to handle RBE requests and brings it up and down
// in loose synchrony with Maintenance Mode.
package foundrybotcustodian

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/recentschannel"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// Start spawns a goroutine that forever brings Foundry Bot up or down in accordance with the
// wishes of machineserver: down during maintenance mode and up otherwise. These wishes are sent on
// wantUpChannel as booleans: true for up and false for down. The caller should send a steady
// heartbeat of these, and we fulfill them as promptly as possible, though allowing Foundry Bot to
// complete its current job before taking it down. We also restart Foundry Bot if it falls down on
// its own.
//
// botPath is the absolute path to a copy of Foundry Bot.
// instance is the GCP instance under which the RBE jobs run. Its project must contain a Remote
//     Build Execution API endpoint under APIs & Services.
//
// Start looks for likely error conditions and returns them before starting the goroutine.
func Start(ctx context.Context, botPath string, instance string, wantUpChannel *recentschannel.Ch[bool]) error {
	// Check as much as we can before spinning off the goroutine.
	_, err := os.Stat(botPath)
	if errors.Is(err, fs.ErrNotExist) {
		return skerr.Wrapf(err, "Foundry Bot not found at %s", botPath)
	}
	if err != nil {
		return skerr.Wrapf(err, "failed to stat() %s", botPath)
	}

	// Start custodianship loop.
	go func() {
		// exits is a channel that receives a message every time the Foundry Bot process ends for any
		// reason. The value is ignored, but it serves as a synchronizer and a provocation to bring the
		// process back up. It's buffered because there's no advantage in waitForProcess() delaying
		// exiting until the custodian receives the value.
		exits := make(chan bool, 1)
		var wantUp bool   // Initial value doesn't matter because we always write before read.
		var cmd *exec.Cmd // cmd is nil iff process is down.

		// Set up metrics.
		const statusMetric = "machine_tmm_foundry_bot_status"
		timeSinceProcessStarted := metrics2.NewLiveness("liveness_machine_tmm_foundry_bot", nil)
		runningMetric := metrics2.GetBoolMetric(statusMetric, map[string]string{"status": "running"})
		maintenanceMetric := metrics2.GetBoolMetric(statusMetric, map[string]string{"status": "maintenance"})
		failedToStartMetric := metrics2.GetBoolMetric(statusMetric, map[string]string{"status": "failed_to_start"})
		failedToStopMetric := metrics2.GetBoolMetric(statusMetric, map[string]string{"status": "failed_to_stop"})

		for {
			select {
			case wantUp = <-wantUpChannel.Recv():
				// A polling request to machineserver has returned.
				switch {
				case wantUp && cmd == nil:
					cmd = startProcess(ctx, botPath, instance, exits, timeSinceProcessStarted)
					// If starting the process failed, we'll have another try at the next heartbeat.
				case !wantUp && cmd != nil:
					cmd = stopProcess(cmd, exits)
				}
			case <-exits:
				// Foundry Bot exited on its own. It's not supposed to do that.
				cmd = nil
				// Start it up again if we like, without waiting for next heartbeat.
				if wantUp {
					cmd = startProcess(ctx, botPath, instance, exits, timeSinceProcessStarted)
					// If starting the process failed, we'll have another try at the next heartbeat.
				}
			case <-ctx.Done():
				// For now, this is an error case, because nobody is canceling the context yet.
				sklog.Infof("Foundry Bot custodian stopped: %s", err)
				return
			}

			// Update metrics.
			runningMetric.Update(wantUp && cmd != nil)
			maintenanceMetric.Update(!wantUp && cmd == nil)
			failedToStartMetric.Update(wantUp && cmd == nil)
			failedToStopMetric.Update(!wantUp && cmd != nil)
		}
	}()
	return nil
}

// startProcess launches the Foundry Bot process and returns a Cmd representing it. It also spins up
// a goroutine to wait for the process to exit; exit codes are sent to the exits channel to prompt a
// listener to restart the process if it likes.
//
// Returns a Cmd representing the process: nil if it wasn't successfully brought up. The process is
// killed when the context is cancelled.
func startProcess(ctx context.Context, botPath string, instance string, exits chan bool, timeSinceProcessStarted metrics2.Liveness) *exec.Cmd {
	// rbeServiceAddress is the FQDN and port of the Foundry service to which the Foundry Bot should
	// connect to receive tasks.
	const rbeServiceAddress = "remotebuildexecution.googleapis.com:443"
	cmd := executil.CommandContext(
		ctx,
		botPath,
		"-service_address="+rbeServiceAddress,
		"-instance_name="+instance,
		"session",
		"-sandbox=none",
		// 6h timeout after sending a SIGINT to foundry_bot because our longest job is about 3.75h.
		// See https://perf.skia.org/e/?queries=sub_result%3Dtask_step_s.
		"-stop_time=6h")
	sklog.Infof("Starting %s", cmd.String())
	timeSinceProcessStarted.Reset()
	err := cmd.Start()
	if err == nil {
		// At this point, the PID is set in the cmd, so it's valid to send signals to it.

		// Spin off a separate goroutine to wait for process exit so we can continue to listen for
		// transitions into maintenance mode on this one.
		go waitForProcess(cmd, exits)

		return cmd
	} else {
		sklog.Errorf("Starting Foundry Bot process failed: %s", err)

		// If starting the command failed, we'll have another try at the next aspiration polling.
		return nil
	}
}

// waitForProcess waits for the Foundry Bot process to exit (whether by signal or normal completion)
// and notifies a channel when it does so a listener can restart the it if desired. Intended to be
// spawned as a new goroutine.
func waitForProcess(cmd *exec.Cmd, exits chan bool) {
	err := cmd.Wait()
	if err == nil {
		sklog.Errorf("Foundry Bot exited without an error, which is unexpected in production.")
	}
	// If the context was canceled, err is an os/exec.ExitError with .Success() == false. It can
	// also be a plain old error if something more exotic goes wrong.
	exits <- false
}

// stopProcess attempts to gracefully stop the Foundry Bot process and waits for it to exit. Returns
// the passed-in Cmd if the process is still up afterward (which it can be due to errors), nil
// otherwise.
func stopProcess(cmd *exec.Cmd, exits chan bool) *exec.Cmd {
	// TODO(erikrose): On Windows, send a CTRL_CLOSE_EVENT, as that gives the
	// program a chance to exit gracefully. First, confirm Foundry Bot listens to that.
	err := cmd.Process.Signal(os.Interrupt)
	if err == nil {
		// If interrupt was sent nicely, wait for process to exit. Signal()'s
		// error conditions are not specified in the Golang docs, but the UNIX implementation's
		// source suggests that, if the process has already exited, an error will be returned.
		<-exits
		return nil
	} else {
		// The process may already have exited on its own. We'll update isUp properly in the
		// "<- exits" case of StartCustodian().
		sklog.Warningf("Sending interrupt signal to Foundry Bot failed: %s", err)
		return cmd
	}
}
