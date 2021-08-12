package main

// hotspare is a runnable that polls an address:port at regular intervals and brings
// up a virtual ip address (using ip command) if address:port is unreachable.

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

var (
	serviceAccountPath = flag.String("service_account_path", "", "Path to the service account.  Can be empty string to use defaults or project metadata")

	virtualIp        = flag.String("virtual_ip", "192.168.1.200", "The virtual ip address that should be brought up if the liveness test fails.")
	virtualInterface = flag.String("virtual_interface", "eth0:0", "The virtual interface that is brought up with the virtual ip address.")

	livenessAddr      = flag.String("liveness_addr", "192.168.1.199:22", "The ip address and port that should be checked for liveness.")
	livenessPeriod    = flag.Duration("liveness_period", time.Second, "How often to test the livenessAddr")
	livenessTimeout   = flag.Duration("liveness_timeout", time.Second, "How long to wait for the livenessAddr to respond/connect.")
	livenessThreshold = flag.Int("liveness_threshold", 5, "How many liveness failures in a row constitute the livenessAddr being down.")

	syncPeriod     = flag.Duration("sync_period", time.Minute, "How often to sync the image from syncPath.")
	syncRemotePath = flag.String("sync_remote_path", "", `Where the image is stored on the remote machine.  This should include ip address.  E.g. "192.168.1.198:/opt/rpi_img/prod.img"`)
	syncLocalPath  = flag.String("sync_local_path", "/opt/rpi_img/prod.img", "Where the image is stored on the local disk.")

	promPort      = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	metricsPeriod = flag.Duration("metrics_period", 5*time.Second, "How often to update metrics on whether or not we are serving.")

	startServingPlaybook = flag.String("start_serving_playbook", "", "The Ansible playbook that, when run locally, will start serving the image.  This should be idempotent.")
	stopServingPlaybook  = flag.String("stop_serving_playbook", "", "The Ansible playbook that, when run locally, will stop serving the image.  This should be idempotent.")

	local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

const FORGIVENESS_THRESHOLD = 100

type virtualIPManager struct {
	Addr                 string
	Period               time.Duration
	Timeout              time.Duration
	Threshold            int
	accumulatedFailures  int
	consecutiveSuccesses int
}

func NewVirtualIPManager(addr string, period, timeout time.Duration, threshold int) *virtualIPManager {
	return &virtualIPManager{
		Addr:                 addr,
		Period:               period,
		Timeout:              timeout,
		Threshold:            threshold,
		accumulatedFailures:  0,
		consecutiveSuccesses: 0,
	}
}

func (v *virtualIPManager) Run(ctx context.Context) {
	for range time.Tick(v.Period) {
		conn, err := net.DialTimeout("tcp", v.Addr, v.Timeout)
		if err != nil {
			sklog.Errorf("Had problem connecting to %s: %s", v.Addr, err)
			v.consecutiveSuccesses = 0
			v.accumulatedFailures++
			if v.accumulatedFailures == v.Threshold {
				bringUpVIP(ctx)
			}
		} else {
			sklog.Infof("Connected successfully to %s. %v\n", v.Addr, conn.Close())
			v.consecutiveSuccesses++
			if v.consecutiveSuccesses >= FORGIVENESS_THRESHOLD {
				v.consecutiveSuccesses = 0
				if v.accumulatedFailures < v.Threshold {
					// Forgive the occasional failures that
					// happen, but not if we went over the
					// threshold (and are currently serving).
					// Occasional failures happen every so often just
					// due to normal network flakes. We want to
					// prevent those occasional flakes from tripping
					// the hotspare.
					v.accumulatedFailures = 0
				}
			}
		}
		metrics2.GetInt64Metric("skolo_hotspare_consecutive_failures", nil).Update(int64(v.accumulatedFailures))
	}
}

func isServing(ctx context.Context) bool {
	out, err := exec.RunSimple(ctx, "ip address")
	if err != nil {
		sklog.Errorf("There was a problem running 'ip address': %s", err)
	}
	return strings.Contains(out, *virtualInterface)
}

func bringUpVIP(ctx context.Context) {
	sklog.Infof("Bringing up VIP, master is dead")
	cmd := fmt.Sprintf("sudo ip address add %s dev %s", *virtualIp, *virtualInterface)
	out, err := exec.RunSimple(ctx, cmd)
	sklog.Infof("Output: %s", out)
	if err != nil {
		sklog.Errorf("Could not bring up VIP: %s", err)
	}

}

type imageSyncer struct {
	Period     time.Duration
	RemotePath string
	LocalPath  string
}

func NewImageSyncer(period time.Duration, remotePath, localPath string) *imageSyncer {
	return &imageSyncer{
		Period:     period,
		RemotePath: remotePath,
		LocalPath:  localPath,
	}
}

func (i *imageSyncer) Run(ctx context.Context) {
	// Force a sync at the beginning to make sure we are in a good state for serving.
	i.sync(ctx)
	for range time.Tick(i.Period) {
		i.sync(ctx)
	}
}

// sync pulls the image from master and then, if successful, reloads the image
// that is being served via NFS.
func (i *imageSyncer) sync(ctx context.Context) bool {
	if isServing(ctx) {
		sklog.Infof("Skipping sync because we are already serving")
		return false
	}

	sklog.Infof("Attempting to sync image from remote")
	stdOut := bytes.Buffer{}
	stdErr := bytes.Buffer{}
	tempDest := i.LocalPath + ".tmp"
	// This only works if the master has the spare's ssh key in authorized_key
	err := exec.Run(ctx, &exec.Command{
		Name:   "rsync",
		Args:   []string{i.RemotePath, tempDest},
		Stdout: &stdOut,
		Stderr: &stdErr,
	})
	sklog.Infof("StdOut of rsync command: %s", stdOut.String())
	sklog.Infof("StdErr of rsync command: %s", stdErr.String())
	if err != nil {
		sklog.Errorf("Could not copy image with rsync. Staying with old image: %s", err)
		return false
	} else {
		sklog.Infof("No error with rsync")
		stopServing(ctx)
		time.Sleep(time.Second) // Make sure old file handle is released.
		if err = os.Rename(tempDest, i.LocalPath); err != nil {
			sklog.Errorf("Could not rename temporary image. Staying with old image: %s", err)
		}
		startServing(ctx)
	}
	return true
}

// stopServing uses Ansible playbooks to stop serving the image.
func stopServing(ctx context.Context) {
	stdOut := bytes.Buffer{}
	stdErr := bytes.Buffer{}
	err := exec.Run(ctx, &exec.Command{
		Name:   "ansible-playbook",
		Args:   []string{"-i", "localhost,", "-c", "local", *stopServingPlaybook},
		Stdout: &stdOut,
		Stderr: &stdErr,
	})
	sklog.Infof("StdOut of Ansible stop command: %s", stdOut.String())
	sklog.Infof("StdErr of Ansible stop command: %s", stdErr.String())
	if err != nil {
		sklog.Errorf("Could not stop serving image: %s", err)
	} else {
		sklog.Infof("Ansible stop serving playbook success")
	}
}

// startServing uses Ansible playbooks to start serving the image,
// which forces a refresh of the image being served.
func startServing(ctx context.Context) {
	stdOut := bytes.Buffer{}
	stdErr := bytes.Buffer{}
	err := exec.Run(ctx, &exec.Command{
		Name:   "ansible-playbook",
		Args:   []string{"-i", "localhost,", "-c", "local", *startServingPlaybook},
		Stdout: &stdOut,
		Stderr: &stdErr,
	})
	sklog.Infof("StdOut of Ansible start command: %s", stdOut.String())
	sklog.Infof("StdErr of Ansible start command: %s", stdErr.String())
	if err != nil {
		sklog.Errorf("Could not start serving image: %s", err)
	} else {
		sklog.Infof("Ansible start serving playbook success")
	}
}

func main() {
	common.InitWithMust(
		"hotspare",
		common.PrometheusOpt(promPort),
		common.CloudLogging(local, "google.com:skia-buildbots"),
	)
	ctx := context.Background()
	lt := NewVirtualIPManager(*livenessAddr, *livenessPeriod, *livenessTimeout, *livenessThreshold)
	go lt.Run(ctx)

	is := NewImageSyncer(*syncPeriod, *syncRemotePath, *syncLocalPath)
	go is.Run(ctx)

	go func() {
		for range time.Tick(*metricsPeriod) {
			if isServing(ctx) {
				metrics2.GetInt64Metric("skolo_hotspare_spare_active", nil).Update(int64(1))
			} else {
				metrics2.GetInt64Metric("skolo_hotspare_spare_active", nil).Update(int64(0))
			}
		}
	}()

	select {}
}
