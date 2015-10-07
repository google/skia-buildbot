package buildbot

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"reflect"
	"strconv"
	"time"

	"go.skia.org/infra/go/util"
)

// buildSlaveByHost represents information about a buildslave which is relevant
// to its host machine.
type BuildSlaveByHost struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	IsInternal bool   `json:"isInternal"`
}

func (s *BuildSlaveByHost) UnmarshalJSON(data []byte) error {
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	if len(arr) != 3 {
		return fmt.Errorf("Unable to parse Slave from JSON; expected array of length 3 but got: %v", arr)
	}
	if reflect.TypeOf(arr[0]).Kind() != reflect.String {
		return fmt.Errorf("Expected: [<string>, <string>, <bool>]")
	}
	s.Name = arr[0].(string)

	if reflect.TypeOf(arr[1]).Kind() != reflect.String {
		return fmt.Errorf("Expected: [<string>, <string>, <bool>]")
	}
	i, err := strconv.ParseInt(arr[1].(string), 10, 32)
	if err != nil {
		return fmt.Errorf("Slave index must be an int: %v", err)
	}
	s.Index = int(i)

	if reflect.TypeOf(arr[2]).Kind() != reflect.Bool {
		return fmt.Errorf("Expected: [<string>, <string>, <bool>]")
	}
	s.IsInternal = arr[2].(bool)
	return nil
}

// SlaveHostCopy represents a file to copy on the host machine before launching slaves.
type SlaveHostCopy struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// SlaveHost represents a host machine which runs some number of buildslaves.
type SlaveHost struct {
	PathToBuildbot []string            `json:"path_to_buildbot"`
	KvmSwitch      string              `json:"kvm_switch"`
	KvmNum         string              `json:"kvm_num"`
	LaunchScript   []string            `json:"launch_script"`
	IP             string              `json:"ip"`
	Copies         []*SlaveHostCopy    `json:"copies"`
	Slaves         []*BuildSlaveByHost `json:"slaves"`
}

// GetSlaveHostsCfg retrieves the slave_hosts_cfg from the repository.
func GetSlaveHostsCfg(workdir string) (map[string]*SlaveHost, error) {
	script := path.Join(workdir, "site_config/slave_hosts_cfg.py")
	output, err := exec.Command("python", script).Output()
	if err != nil {
		return nil, err
	}
	rv := map[string]*SlaveHost{}
	if err := json.Unmarshal(output, &rv); err != nil {
		return nil, err
	}
	return rv, nil
}

// SlaveHostsCfgPoller periodically reads the slave_hosts_cfg.py file from the
// repo. It does NOT update the repository; it is assumed that the caller takes
// care of that.
func SlaveHostsCfgPoller(workdir string) (*util.PollingStatus, error) {
	return util.NewPollingStatus(func() (interface{}, error) {
		return GetSlaveHostsCfg(workdir)
	}, 5*time.Minute)
}
