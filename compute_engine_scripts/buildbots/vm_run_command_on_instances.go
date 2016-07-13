package main

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
)

/*
	Run a specified command on all specified Skia GCE instances.
*/

var (
	user         = flag.String("user", "default", "The user who should run the command")
	cmd          = flag.String("cmd", "", "The command to run")
	gcomputeCmd  = flag.String("gcompute_cmd", "gcloud compute", "Command used to SSH into the GCE instances.")
	vmNamePrefix = flag.String("vm_name_prefix", "skia-vm", "Prefix for VM names.")
	rangeStart   = flag.Int("range_start", 0, "Bot range start, inclusive")
	rangeEnd     = flag.Int("range_end", 0, "Bot range end, inclusive")
	verbose      = flag.Bool("verbose", false, "Print all output from commands.")
)

func main() {
	defer common.LogPanic()
	common.Init()

	if *cmd == "" {
		glog.Fatalf("--cmd is required.")
	}

	instances := []string{}
	for i := *rangeStart; i <= *rangeEnd; i++ {
		instances = append(instances, fmt.Sprintf("%s-%03d", *vmNamePrefix, i))
	}
	outputs := map[string]string{}
	errs := map[string]error{}
	mtx := sync.Mutex{}

	var wg sync.WaitGroup
	for _, instanceName := range instances {
		wg.Add(1)
		go func(instanceName string) {
			defer wg.Done()
			cmdLine := fmt.Sprintf("%s ssh --ssh_user=%s %s %s", *gcomputeCmd, *user, instanceName, *cmd)
			split := strings.Fields(cmdLine)
			name := split[0]
			args := split[1:]
			c := exec.Command(name, args...)
			output, err := c.CombinedOutput()
			mtx.Lock()
			defer mtx.Unlock()
			if err != nil {
				errs[instanceName] = err
			}
			outputs[instanceName] = string(output)
		}(instanceName)
	}
	wg.Wait()

	for _, instanceName := range instances {
		err, _ := errs[instanceName]
		if *verbose || err != nil {
			glog.Infof("========== %s ==========", instanceName)
			if err != nil {
				glog.Infof("Command failed: %s\n", err)
			}
			glog.Infof(outputs[instanceName])
			glog.Infof("====================================")
		}
	}
}
