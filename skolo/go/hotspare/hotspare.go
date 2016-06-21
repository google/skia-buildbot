package main

// hotspare is a runnable that polls an address:port at regular intervals and brings
// up a virtual ip address (using ifconfig) if address:port is unreachable.
// TODO(kjlubick): Add cloud logging support skia:5447

import (
	"flag"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
)

var (
	virtualIp        = flag.String("virtual_ip", "192.168.1.200", "The virtual ip address that should be brought up if the liveness test fails.")
	virtualInterface = flag.String("virtual_interface", "eth0:0", "The virtual interface that is brought up with the virtual ip address.")

	livenessAddr      = flag.String("liveness_addr", "192.168.1.199:22", "The ip address and port that should be checked for liveness.")
	livenessPeriod    = flag.Duration("liveness_period", time.Second, "How often to test the livenessAddr")
	livenessTimeout   = flag.Duration("liveness_timeout", time.Second, "How long to wait for the livenessAddr to respond/connect.")
	livenessThreshold = flag.Int("liveness_threshold", 5, "How many liveness failures in a row constitute the livenessAddr being down.")

	syncPeriod     = flag.Duration("sync_period", time.Minute, "How often to sync the image from syncPath.")
	syncRemotePath = flag.String("sync_remote_path", "", `Where the image is stored on the remote machine.  This should include ip address.  E.g. "192.168.1.198:/opt/rpi_img/current.img"`)
	syncLocalPath  = flag.String("sync_local_path", "/opt/rpi_img/current.img", "Where the image is stored on the local disk.")
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
			glog.Errorf("Had problem connecting to %s: %s", v.Addr, err)
			v.consecutiveFailures++
			if v.consecutiveFailures == v.Threshold {
				bringUpVIP()
			}
		} else {
			glog.Infof("Connected successfully to %s: %v\n", v.Addr, conn.Close())
			if v.consecutiveFailures >= v.Threshold {
				tearDownVIP()
			}
			v.consecutiveFailures = 0
		}
	}
}

func isServing() bool {
	out, err := exec.RunSimple("ifconfig")
	if err != nil {
		glog.Errorf("There was a problem running ifconfig: %s", err)
	}
	return strings.Contains(out, *virtualInterface)
}

func bringUpVIP() {
	glog.Infof("Bringing up VIP, master is dead")
	cmd := fmt.Sprintf("sudo ifconfig %s %s", *virtualInterface, *virtualIp)
	out, err := exec.RunSimple(cmd)
	glog.Infof("Output: %s", out)
	glog.Errorf("Error: %v", err)
}

func tearDownVIP() {
	glog.Infof("Tearing down VIP, master is live")
	cmd := fmt.Sprintf("sudo ifconfig %s down", *virtualInterface)
	out, err := exec.RunSimple(cmd)
	glog.Infof("Output: %s", out)
	glog.Errorf("Error: %v", err)
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
			glog.Infof("Skipping sync because we are serving")
			continue
		}
		glog.Infof("Attempting to sync image from remote")
		// This only works if the master has the spare's ssh key in authorized_key
		err := exec.Run(&exec.Command{
			Name:      "scp",
			Args:      []string{i.RemotePath, i.LocalPath},
			LogStderr: true,
			LogStdout: true,
		})
		glog.Errorf("Error: %s", err)
	}
}

func main() {
	defer common.LogPanic()
	common.Init()

	lt := NewVirtualIPManager(*livenessAddr, *livenessPeriod, *livenessTimeout, *livenessThreshold)
	go lt.Run()

	is := NewImageSyncer(*syncPeriod, *syncRemotePath, *syncLocalPath)
	go is.Run()

	select {}
}
