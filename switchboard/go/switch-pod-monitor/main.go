// switch-pod-monitor is the main application running in a switch-pod,
// registering the pod with switchboard and periodically updating the pods
// LastUpdated value. It will also remove the pod from switchboard when the
// kubernetes tries to shutdown the pod. See http://go/skia-switchboard for more
// details.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/switchboard"
)

var (
	configFlag = flag.String("config", "../machine/go/configs/test.json", "The path to the configuration file.")
	local      = flag.Bool("local", false, "Running locally if true, as opposed to running in GCE.")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

func main() {
	// We don't use common.Init() here because we want to register our own signal handlers.
	flag.Parse()
	metrics2.InitPrometheus(*promPort)
	ctx := context.Background()

	// Load config file.
	var instanceConfig config.InstanceConfig
	err := util.WithReadFile(*configFlag, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if err != nil {
		sklog.Fatalf("Failed to open config file: %q: %s", *configFlag, err)
	}

	// Create Switchboard instance.
	switchboardInstance, err := switchboard.New(ctx, *local, instanceConfig)
	if err != nil {
		sklog.Fatalf("Failed to initialize Switchboard instance: %s", err)
	}

	// Find hostname.
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatalf("Failed to load hostname: %s", err)
	}

	if err := connectToSwitchboardAndWait(ctx, hostname, switchboardInstance, switchboard.PodKeepAliveDuration, switchboard.PodMaxConsecutiveKeepAliveErrors); err != nil {
		sklog.Fatal(err)
	}
}

// connectToSwitchboardAndWait registers the pod with switchboard and does not
// return unless the given context is cancelled.
//
// keepAliveDuration is how often to call Switchboard.KeepAlivePod.
//
// maxConsecutiveKeepAliveErrors is the number of time Switchboard.KeepAlivePod
// calls are allowed to fail consecutively before returning an error.
func connectToSwitchboardAndWait(ctx context.Context, hostname string, switchboardInstance switchboard.Switchboard, keepAliveDuration time.Duration, maxConsecutiveKeepAliveErrors int) error {
	// Kubernetes will send a SIGTERM when it wants to tell a pod it is going to
	// be shutdown. After waiting for the graceful shutdown period, if the pod
	// is still running it will be sent a SIGKILL. See
	// https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination
	// for more details.
	//
	// On syscall.SIGTERM we call Switchboard.RemovePod so this pod doesn't handle
	// any more new switchboard connections. We then use the k8s grace period to wait
	// for all the existing leases for machines to expire. At the end of the graceperiod
	// the pod is killed.
	//
	// We also handle syscall.SIGINT so that we can kill the app using Ctrl-C
	// when running on the desktop and RemovePod still gets called.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	if err := switchboardInstance.AddPod(ctx, hostname); err != nil {
		return skerr.Wrapf(err, "Failed to add pod: %q", hostname)
	}

	consecutiveFailures := 0

	// Are we in the graceful shutdown phase of the pod lifecycle?
	gracefulShutdown := false

	for {
		select {
		case <-ctx.Done():
			return nil
		case sig := <-c:
			sklog.Error("Got signal:", sig)
			if err := switchboardInstance.RemovePod(ctx, hostname); err != nil {
				sklog.Error(err)
			}
			gracefulShutdown = true
			if sig == syscall.SIGINT {
				sklog.Info("Exiting on SIGINT.")
				os.Exit(0)
			}
		// Periodically call KeepAlivePod so we know the pod is still running.
		case <-time.Tick(keepAliveDuration):
			if gracefulShutdown {
				// Check to see if there are any meeting points left in this pod
				// that is, if any tasks are using test machines connected to
				// this pod. If there are none then exit.
				count, err := switchboardInstance.NumMeetingPointsForPod(ctx, hostname)
				if err != nil {
					sklog.Errorf("switchboard failed to count meeting points: %s", err)
					continue
				}
				sklog.Infof("graceful shutdown mode for pod: %q num meeting points: %s", hostname, count)
				if count == 0 {
					return nil
				}
			} else {
				err := switchboardInstance.KeepAlivePod(ctx, hostname)
				if err != nil {
					consecutiveFailures++
					sklog.Errorf("Failed to keep pod alive: %s", err)
					if consecutiveFailures >= maxConsecutiveKeepAliveErrors {
						if err := switchboardInstance.RemovePod(ctx, hostname); err != nil {
							sklog.Error(err)
						}
						return skerr.Wrapf(err, "Exiting: Switchpod.KeepAlivePod failed %d consecutive times", consecutiveFailures)
					}
				}
			}
			consecutiveFailures = 0
		}
	}
}
