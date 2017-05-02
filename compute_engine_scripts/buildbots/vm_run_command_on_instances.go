package main

import (
	"flag"
	"fmt"
	"os/exec"
	"sync"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

/*
	Run a specified command on all specified Skia GCE instances.
*/

var (
	user         = flag.String("user", "default", "The user who should run the command")
	cmd          = flag.String("cmd", "", "The command to run")
	project      = flag.String("project", "google.com:skia-buildbots", "GCE project ID of the VMs.")
	zone         = flag.String("zone", "us-central1-c", "Zone of the VMs.")
	vmNamePrefix = flag.String("vm_name_prefix", "skia-vm", "Prefix for VM names.")
	rangeStart   = flag.Int("range_start", 0, "Bot range start, inclusive")
	rangeEnd     = flag.Int("range_end", 0, "Bot range end, inclusive")
	verbose      = flag.Bool("verbose", false, "Print all output from commands.")
)

func main() {
	defer common.LogPanic()
	common.Init()

	if *cmd == "" {
		sklog.Fatalf("--cmd is required.")
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
			c := exec.Command("gcloud", "compute", "--project", *project, "ssh", "--zone", *zone, "--command", *cmd, fmt.Sprintf("%s@%s", *user, instanceName))
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
			sklog.Infof("========== %s ==========", instanceName)
			if err != nil {
				sklog.Infof("Command failed: %s\n", err)
			}
			sklog.Infof(outputs[instanceName])
			sklog.Infof("====================================")
		}
	}
}
