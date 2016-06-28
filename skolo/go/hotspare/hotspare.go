package main

// hotspare is a runnable that polls an address:port at regular intervals and brings
// up a virtual ip address (using ifconfig) if address:port is unreachable.

import (
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
	for range time.Tick(i.Period) {
		if isServing() {
			sklog.Infof("Skipping sync because we are serving")
			continue
		}
		sklog.Infof("Attempting to sync image from remote")
		// This only works if the master has the spare's ssh key in authorized_key
		err := exec.Run(&exec.Command{
			Name:      "scp",
			Args:      []string{i.RemotePath, i.LocalPath},
			LogStderr: true,
			LogStdout: true,
		})
		if err != nil {
			sklog.Errorf("Could not SCP: %s", err)
		} else {
			sklog.Infof("No error with scp")
		}

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
