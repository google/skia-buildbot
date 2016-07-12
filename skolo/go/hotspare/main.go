package main

// hotspare is a runnable that polls an address:port at regular intervals and brings
// up a virtual ip address (using ifconfig) if address:port is unreachable.

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/influxdb"
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
	syncRemotePath = flag.String("sync_remote_path", "", `Where the image is stored on the remote machine.  This should include ip address.  E.g. "192.168.1.198:/opt/rpi_img/current.img"`)
	syncLocalPath  = flag.String("sync_local_path", "/opt/rpi_img/current.img", "Where the image is stored on the local disk.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")

	startServingPlaybook = flag.String("start_serving_playbook", "", "The Ansible playbook that, when run locally, will start serving the image.  This should be idempotent.")
	stopServingPlaybook  = flag.String("stop_serving_playbook", "", "The Ansible playbook that, when run locally, will stop serving the image.  This should be idempotent.")
)

type virtualIPManager struct {
	Addr                string
	Period              time.Duration
	Timeout             time.Duration
	Threshold           int
	consecutiveFailures int
}

func NewVirtualIPManager(addr string, period, timeout time.Duration, threshold int) *virtualIPManager {
	return &virtualIPManager{
		Addr:                addr,
		Period:              period,
		Timeout:             timeout,
		Threshold:           threshold,
		consecutiveFailures: 0,
	}
}

func (v *virtualIPManager) Run() {
	for range time.Tick(v.Period) {
		conn, err := net.DialTimeout("tcp", v.Addr, v.Timeout)
		if err != nil {
			sklog.Errorf("Had problem connecting to %s: %s", v.Addr, err)
			v.consecutiveFailures++
			if v.consecutiveFailures == v.Threshold {
				bringUpVIP()
			}
		} else {
			sklog.Infof("Connected successfully to %s: %v\n", v.Addr, conn.Close())
			if v.consecutiveFailures >= v.Threshold {
				tearDownVIP()
			}
			v.consecutiveFailures = 0
		}
		metrics2.GetInt64Metric("skolo.hotspare.consecutive_failures", nil).Update(int64(v.consecutiveFailures))
	}
}

func isServing() bool {
	out, err := exec.RunSimple("ifconfig")
	if err != nil {
		sklog.Errorf("There was a problem running ifconfig: %s", err)
	}
	return strings.Contains(out, *virtualInterface)
}

func bringUpVIP() {
	sklog.Infof("Bringing up VIP, master is dead")
	cmd := fmt.Sprintf("sudo ifconfig %s %s", *virtualInterface, *virtualIp)
	out, err := exec.RunSimple(cmd)
	sklog.Infof("Output: %s", out)
	if err != nil {
		sklog.Errorf("Could not bring up VIP: %s", err)
	}
}

func tearDownVIP() {
	sklog.Infof("Tearing down VIP, master is live")
	cmd := fmt.Sprintf("sudo ifconfig %s down", *virtualInterface)
	out, err := exec.RunSimple(cmd)
	sklog.Infof("Output: %s", out)
	if err != nil {
		sklog.Errorf("Could not tear down VIP: %s", err)
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

func (i *imageSyncer) Run() {
	// Force a sync at the beginning to make sure we are in a good state for serving.
	i.sync()
	for range time.Tick(i.Period) {
		i.sync()
	}
}

// sync pulls the image from master and then, if successful, reloads the image
// that is being served via NFS.
func (i *imageSyncer) sync() bool {
	if isServing() {
		sklog.Infof("Skipping sync because we are already serving")
		return false
	}
	sklog.Infof("Attempting to sync image from remote")
	stdOut := bytes.Buffer{}
	stdErr := bytes.Buffer{}
	// This only works if the master has the spare's ssh key in authorized_key
	err := exec.Run(&exec.Command{
		Name:   "rsync",
		Args:   []string{i.RemotePath, i.LocalPath},
		Stdout: &stdOut,
		Stderr: &stdErr,
	})
	sklog.Infof("StdOut of rsync command: %s", stdOut.String())
	sklog.Infof("StdErr of rsync command: %s", stdErr.String())
	if err != nil {
		sklog.Errorf("Could not copy image with rsync: %s", err)
		return false
	} else {
		sklog.Infof("No error with rsync")
		reloadImage()
	}
	return true
}

// reloadImage uses Ansible playbooks to stop and then quickly start serving the image,
// which forces a refresh of the image being served.
func reloadImage() {
	stdOut := bytes.Buffer{}
	stdErr := bytes.Buffer{}
	err := exec.Run(&exec.Command{
		Name:   "ansible-playbook",
		Args:   []string{"-i", `"localhost,"`, "-c", "local", *stopServingPlaybook},
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

	stdOut.Reset()
	stdErr.Reset()
	err = exec.Run(&exec.Command{
		Name:   "ansible-playbook",
		Args:   []string{"-i", `"localhost,"`, "-c", "local", *startServingPlaybook},
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
	defer common.LogPanic()
	common.InitExternalWithMetrics2("hotspare", influxHost, influxUser, influxPassword, influxDatabase)

	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountPath, nil, sklog.CLOUD_LOGGING_WRITE_SCOPE)
	if err != nil {
		sklog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}

	err = sklog.InitCloudLogging(client, "rpi-master", "hotspare")
	if err != nil {
		sklog.Fatalf("Could not setup cloud sklog: %s", err)
	}

	lt := NewVirtualIPManager(*livenessAddr, *livenessPeriod, *livenessTimeout, *livenessThreshold)
	go lt.Run()

	is := NewImageSyncer(*syncPeriod, *syncRemotePath, *syncLocalPath)
	go is.Run()

	select {}
}
