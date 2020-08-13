package main

/*
   Program for automating creation and setup of Swarming bot VMs.

   Bot numbers should be assigned as follows:
     1-99 (skia-e-gce-0..): Temporary or experimental bots.
     100-499 (skia-e-gce-[1234]..): Linux
       100-199: linux-small
       200-299: linux-medium
       300-399: linux-large
       400-499: linux-skylake
     500-699 (skia-e-gce-[56]..): Windows
       500-599: win-medium
       600-699: win-large
     700-999: unassigned
*/

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/swarming/instance_types"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags.
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	dev            = flag.Bool("dev", false, "Whether or not the bots connect to chromium-swarm-dev.")
	dumpJson       = flag.Bool("dump-json", false, "Dump out JSON for each of the instances to create/delete and exit without changing anything.")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	instanceType   = flag.String("type", "", fmt.Sprintf("Type of instance; one of: %v", instance_types.VALID_INSTANCE_TYPES))
	internal       = flag.Bool("internal", false, "Whether or not the bots are internal.")
	workdir        = flag.String("workdir", ".", "Working directory.")
)

func main() {
	common.Init()

	// Validation.
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
	}
	if !util.In(*instanceType, instance_types.VALID_INSTANCE_TYPES) {
		sklog.Fatalf("--type must be one of %v", instance_types.VALID_INSTANCE_TYPES)
	}
	instanceNums, err := util.ParseIntSet(*instances)
	if err != nil {
		sklog.Fatal(err)
	}
	if len(instanceNums) == 0 {
		sklog.Fatal("Please specify at least one instance number via --instances.")
	}

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Setup.
	_, filename, _, _ := runtime.Caller(0)
	checkoutRoot := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
	ctx := context.Background()

	getInstance, err := instance_types.GetInstanceConstructor(ctx, *instanceType, checkoutRoot, wdAbs, *internal, *dev)
	if err != nil {
		sklog.Fatal(err)
	}
	project, zone := instance_types.GetProjectAndZone(*instanceType, *internal)

	// Create the GCloud object.
	g, err := gce.NewLocalGCloud(project, zone)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := g.CheckSsh(); err != nil {
		sklog.Fatal(err)
	}

	// Create the Instance objects.
	vms := make([]*gce.Instance, 0, len(instanceNums))
	for _, num := range instanceNums {
		vms = append(vms, getInstance(num))
	}

	// If requested, dump JSON for the given instances and exit.
	if *dumpJson {
		verb := "create"
		if *delete {
			verb = "delete"
		}
		data := map[string][]*gce.Instance{
			verb: vms,
		}
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			sklog.Fatal(err)
		}
		sklog.Infof("\n%s", string(b))
		return
	}

	// Perform the requested operation.
	verb := "Creating"
	if *delete {
		verb = "Deleting"
	}
	sklog.Infof("%s instances: %v", verb, instanceNums)
	group := util.NewNamedErrGroup()
	for _, vm := range vms {
		vm := vm // https://golang.org/doc/faq#closures_and_goroutines
		group.Go(vm.Name, func() error {
			if *create {
				if err := g.CreateAndSetup(ctx, vm, *ignoreExists); err != nil {
					return err
				}

				if strings.Contains(vm.Os, "Win") {
					if err := g.WaitForLogMessage(vm, "*** Start Swarming. ***", 7*time.Minute); err != nil {
						return err
					}
				}
			} else {
				return g.Delete(vm, *ignoreExists, *deleteDataDisk)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		sklog.Fatal(err)
	}
}
