package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
	"golang.org/x/crypto/ssh"
)

const (
	KEY_FILE           = "id_rsa"
	WORKER_NUM_KEYWORD = "{{worker_num}}"
)

type workerResp struct {
	hostname string
	output   string
}

func executeCmd(cmd, hostname string, config *ssh.ClientConfig, timeout time.Duration) (string, error) {
	// Dial up TCP connection to remote machine.
	conn, err := net.Dial("tcp", hostname+":22")
	if err != nil {
		return "", fmt.Errorf("Failed to ssh connect to %s. Make sure \"PubkeyAuthentication yes\" is in your sshd_config: %s", hostname, err)
	}
	defer util.Close(conn)
	util.LogErr(conn.SetDeadline(time.Now().Add(timeout)))

	// Create new SSH client connection.
	sshConn, sshChan, req, err := ssh.NewClientConn(conn, hostname+":22", config)
	if err != nil {
		return "", fmt.Errorf("Failed to ssh connect to %s: %s", hostname, err)
	}
	// Use client connection to create new client.
	client := ssh.NewClient(sshConn, sshChan, req)

	// Client connections can support multiple interactive sessions.
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("Failed to ssh connect to %s: %s", hostname, err)
	}

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("Errored or Timeout out while running \"%s\" on %s: %s", cmd, hostname, err)
	}
	return stdoutBuf.String(), nil
}

func getKeyFile() (key ssh.Signer, err error) {
	usr, _ := user.Current()
	file := usr.HomeDir + "/.ssh/" + KEY_FILE
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	key, err = ssh.ParsePrivateKey(buf)
	if err != nil {
		return
	}
	return
}

// SSH connects to the specified workers and runs the specified command. If the
// command does not complete in the given duration then all remaining workers are
// considered timed out. SSH also automatically substitutes the sequential number
// of the worker for the WORKER_NUM_KEYWORD since it is a common use case.
func SSH(cmd string, workers []string, timeout time.Duration) (map[string]string, error) {
	glog.Infof("Running \"%s\" on %s with timeout of %s", cmd, workers, timeout)

	// Ensure that the key file exists.
	key, err := getKeyFile()
	if err != nil {
		return nil, fmt.Errorf("Failed to get key file: %s", err)
	}

	// Initialize the structure with the configuration for ssh.
	config := &ssh.ClientConfig{
		User: CT_USER,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	var wg sync.WaitGroup
	// m protects workersWithOutputs and remainingWorkers
	var m sync.Mutex
	// Will be populated and returned by this function.
	workersWithOutputs := map[string]string{}
	// Keeps track of which workers are still pending.
	remainingWorkers := map[string]int{}

	// Kick off a goroutine on all workers.
	for i, hostname := range workers {
		wg.Add(1)
		m.Lock()
		remainingWorkers[hostname] = 1
		m.Unlock()
		go func(index int, hostname string) {
			defer wg.Done()
			updatedCmd := strings.Replace(cmd, WORKER_NUM_KEYWORD, strconv.Itoa(index+1), -1)
			output, err := executeCmd(updatedCmd, hostname, config, timeout)
			if err != nil {
				glog.Errorf("Could not execute ssh cmd: %s", err)
			}
			m.Lock()
			defer m.Unlock()
			workersWithOutputs[hostname] = output
			delete(remainingWorkers, hostname)
			glog.Infoln()
			glog.Infof("[%d/%d] Worker %s has completed execution", NUM_WORKERS-len(remainingWorkers), NUM_WORKERS, hostname)
			glog.Infof("Remaining workers: %v", remainingWorkers)
		}(i, hostname)
	}

	wg.Wait()
	glog.Infoln()
	glog.Infof("Finished running \"%s\" on all %d workers", cmd, NUM_WORKERS)
	glog.Info("========================================")

	m.Lock()
	defer m.Unlock()
	return workersWithOutputs, nil
}

// RebootWorkers reboots all CT workers and waits for few mins before returning.
func RebootWorkers() {
	if _, err := SSH("sudo reboot", Slaves, REBOOT_TIMEOUT); err != nil {
		glog.Errorf("Got error while rebooting workers: %v", err)
	}
	waitTime := 15 * time.Minute
	glog.Infof("Waiting for %s till all workers come back from reboot", waitTime)
	time.Sleep(waitTime)
}
