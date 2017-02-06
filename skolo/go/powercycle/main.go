package main

// file-backup is an executable that backs up a given file to Google storage.
// It is meant to be run on a timer, e.g. daily.

// Available files
// active_pwr1
// active_pwr2
// active_pwr3
// active_pwr4
// active_pwr5
// active_pwr6
// active_pwr7
// active_pwr8
// cf_count1
// cf_count2
// cf_count3
// cf_count4
// cf_count5
// cf_count6
// cf_count7
// cf_count8
// clear_ae1
// clear_ae2
// clear_ae3
// clear_ae4
// clear_ae5
// clear_ae6
// clear_ae7
// clear_ae8
// current_max
// em3_test
// enabled1
// enabled2
// enabled3
// enabled4
// enabled5
// enabled6
// enabled7
// enabled8
// energy_sum1
// energy_sum2
// energy_sum3
// energy_sum4
// energy_sum5
// energy_sum6
// energy_sum7
// energy_sum8
// i_rms1
// i_rms2
// i_rms3
// i_rms4
// i_rms5
// i_rms6
// i_rms7
// i_rms8
// lock1
// lock2
// lock3
// lock4
// lock5
// lock6
// lock7
// lock8
// master1
// master2
// master3
// master4
// master5
// master6
// master7
// master8
// meter_ic_ver1
// meter_ic_ver2
// meter_ic_ver3
// meter_ic_ver4
// meter_ic_ver5
// meter_ic_ver6
// meter_ic_ver7
// meter_ic_ver8
// outlet1
// outlet2
// outlet3
// outlet4
// outlet5
// outlet6
// outlet7
// outlet8
// output1
// output2
// output3
// output4
// output5
// output6
// output7
// output8
// pf1
// pf2
// pf3
// pf4
// pf5
// pf6
// pf7
// pf8
// raw_active_pwr1
// raw_active_pwr2
// raw_active_pwr3
// raw_active_pwr4
// raw_active_pwr5
// raw_active_pwr6
// raw_active_pwr7
// raw_active_pwr8
// raw_i_rms1
// raw_i_rms2
// raw_i_rms3
// raw_i_rms4
// raw_i_rms5
// raw_i_rms6
// raw_i_rms7
// raw_i_rms8
// raw_v_rms1
// raw_v_rms2
// raw_v_rms3
// raw_v_rms4
// raw_v_rms5
// raw_v_rms6
// raw_v_rms7
// raw_v_rms8
// relay1
// relay2
// relay3
// relay4
// relay5
// relay6
// relay7
// relay8
// reset1
// reset2
// reset3
// reset4
// reset5
// reset6
// reset7
// reset8
// spi_err_counter
// v_rms1
// v_rms2
// v_rms3
// v_rms4
// v_rms5
// v_rms6
// v_rms7
// v_rms8
// voltage_max
// voltage_min

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/crypto/ssh"
)

var (
	mPowerAddress = flag.String("mpower_address", "192.168.1.39:22", "IP address and port of the mPower power strip.")
	powerPort     = flag.Int("power_port", -1, "Port to power cycle. Values: 1-8")
)

const MPOWER_ROOT = "testproc/power"
const MPOWER_RELAY = "relay%d"

type MPowerClient struct {
	client *ssh.Client
}

func NewMPowerClient(address string) (*MPowerClient, error) {
	key, err := ioutil.ReadFile("/usr/local/google/home/stephana/.ssh/id_rsa")
	if err != nil {
		return nil, fmt.Errorf("Unable to read private key: %v", err)
	}
	sklog.Infof("Retrieved private key")

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse private key: %v", err)
	}
	sklog.Infof("Parsed private key")

	config := &ssh.ClientConfig{
		User: "chrome-bot",
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}

	sklog.Infof("Signed private key")

	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, err
	}

	sklog.Infof("Dial successful")

	ret := &MPowerClient{
		client: client,
	}

	if err := ret.ping(); err != nil {
		return nil, err
	}

	return ret, nil
}

func (m *MPowerClient) ping() error {
	sklog.Infof("Executing ping.")

	session, err := m.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	out, err := session.CombinedOutput("pwd")
	if err != nil {
		return err
	}
	sklog.Infof("PWD: %s", string(out))
	return nil
}

func (m *MPowerClient) issueCommand(cmd string) error {
	return nil
}

type PortVal int

const (
	PORT_OFF PortVal = 0
	PORT_ON  PortVal = 1
)

var POWER_VALUES = map[string]PortVal{
	"0": PORT_OFF,
	"1": PORT_ON,
}

func (m *MPowerClient) GetPortValue(port int) (PortVal, error) {
	if !m.validPort(port) {
		return PORT_OFF, fmt.Errorf("Invalid port. Expected 1-8 got %d", port)
	}

	session, err := m.client.NewSession()
	if err != nil {
		return PORT_OFF, err
	}
	defer session.Close()

	cmd := fmt.Sprintf("cat %s", m.getRelayFile(port))
	outBytes, err := session.CombinedOutput(cmd)
	if err != nil {
		return PORT_OFF, err
	}
	out := strings.TrimSpace(string(outBytes))
	current, ok := POWER_VALUES[out]
	if !ok {
		return PORT_OFF, fmt.Errorf("Got unexpected relay value: %s", out)
	}
	return current, nil
}

func (m *MPowerClient) getRelayFile(port int) string {
	return path.Join(MPOWER_ROOT, fmt.Sprintf(MPOWER_RELAY, port))
}

func (m *MPowerClient) validPort(port int) bool {
	return (port >= 1) && (port <= 8)
}

func (m *MPowerClient) SetPortValue(port int, newVal PortVal) error {
	if !m.validPort(port) {
		return fmt.Errorf("Invalid port. Expected 1-8 got %d", port)
	}

	session, err := m.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// Check if the value is already the target value.
	if current, err := m.GetPortValue(port); err != nil {
		return err
	} else if current == newVal {
		return nil
	}

	cmd := fmt.Sprintf("echo '%d' > %s", newVal, m.getRelayFile(port))
	_, err = session.CombinedOutput(cmd)
	if err != nil {
		return err
	}

	// Check if the value was set correctly.
	if current, err := m.GetPortValue(port); err != nil {
		return err
	} else if current != newVal {
		return fmt.Errorf("Could not read back new value. Got %d", current)
	}

	return nil
}

func (m *MPowerClient) powerCycle(port int) error {
	if err := m.SetPortValue(port, PORT_OFF); err != nil {
		return err
	}

	sklog.Infof("Switched port %d off. Waiting for 10 seconds.", port)
	time.Sleep(10 * time.Second)
	if err := m.SetPortValue(port, PORT_ON); err != nil {
		return err
	}

	sklog.Infof("Switched port %d on.", port)
	return nil
}

func main() {
	common.Init()
	if (*powerPort < 1) || (*powerPort > 8) {
		sklog.Errorf("Power port must be in range 1-8. Got: %d", *powerPort)
		os.Exit(1)
	}

	client, err := NewMPowerClient(*mPowerAddress)
	if err != nil {
		sklog.Fatalf("Unable to create mPower client. Got error: %s", err)
	}

	if err := client.powerCycle(*powerPort); err != nil {
		sklog.Fatalf("Unable to power cycle port %d. Got error: %s", *powerPort, err)
	}

	sklog.Infof("Power cycle successful. All done.")
	sklog.Flush()
}
