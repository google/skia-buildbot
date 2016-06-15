package main

// hotspare is a runnable that polls an address:port at regular intervals and brings
// up a virtual ip address (using ifconfig) if address:port is unreachable.

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

func (l *virtualIPManager) Run() {
	for range time.Tick(l.Period) {
		conn, err := net.DialTimeout("tcp", l.Addr, l.Timeout)
		if err != nil {
			glog.Errorf("Had problem connecting to %s: %s", l.Addr, err)
			l.consecutiveFailures++
			if l.consecutiveFailures == l.Threshold {
				bringUpVIP()
			}
		} else {
			glog.Infof("Connected successfully to %s: %s\n", l.Addr, conn.Close())
			if l.consecutiveFailures >= l.Threshold {
				tearDownVIP()
			}
			l.consecutiveFailures = 0
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
	cmd := fmt.Sprintf("ifconfig %s %s", *virtualInterface, *virtualIp)
	out, err := exec.RunSimple(cmd)
	glog.Infof("Output: %s", out)
	glog.Errorf("Error: %v", err)
}

func tearDownVIP() {
	glog.Infof("Tearing down VIP, master is live")
	cmd := fmt.Sprintf("ifconfig %s down", *virtualInterface)
	out, err := exec.RunSimple(cmd)
	glog.Infof("Output: %s", out)
	glog.Errorf("Error: %v", err)
}

func main() {
	defer common.LogPanic()
	common.Init()

	lt := NewVirtualIPManager(*livenessAddr, *livenessPeriod, *livenessTimeout, *livenessThreshold)
	go lt.Run()

	select {}
}
