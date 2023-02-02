package main

/*
   Program for automating creation and setup of Swarming bot VMs.
*/

import (
	"context"
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/ct/instance_types"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags.
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	androidBuilder = flag.Bool("android-builder", false, "Whether or not this is an android builder instance.")
	linuxBuilder   = flag.Bool("linux-builder", false, "Whether or not this is a linux builder instance.")
	master         = flag.Bool(git.MasterBranch, false, "Whether or not this is a linux master instance.")
	worker         = flag.Bool("worker", false, "Whether or not this is a linux worker instance.")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
)

func main() {
	common.Init()

	// Validation.
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
	}

	numInstanceTypeFlagsSet := 0
	if *androidBuilder {
		numInstanceTypeFlagsSet++
	}
	if *linuxBuilder {
		numInstanceTypeFlagsSet++
	}
	if *master {
		numInstanceTypeFlagsSet++
	}
	if *worker {
		numInstanceTypeFlagsSet++
	}
	if numInstanceTypeFlagsSet != 1 {
		sklog.Fatal("Must specify exactly one of the builder flags or --master or --worker")
	}

	instanceNums, err := util.ParseIntSet(*instances)
	if err != nil {
		sklog.Fatal(err)
	}
	if len(instanceNums) == 0 {
		sklog.Fatal("Please specify at least one instance number via --instances.")
	}
	verb := "Creating"
	if *delete {
		verb = "Deleting"
	}
	sklog.Infof("%s instances: %v", verb, instanceNums)

	// Create the GCloud object.
	g, err := gce.NewLocalGCloud(gce.PROJECT_ID_CT_SWARMING, gce.ZONE_CT)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := g.CheckSsh(); err != nil {
		sklog.Fatal(err)
	}

	// Perform the requested operation.
	ctx := context.Background()
	group := util.NewNamedErrGroup()
	for _, num := range instanceNums {
		var vm *gce.Instance
		if *androidBuilder {
			vm = instance_types.CTAndroidBuilderInstance(num)
		} else if *linuxBuilder {
			vm = instance_types.CTLinuxBuilderInstance(num)
		} else if *master {
			vm = instance_types.CTMasterInstance(num)
		} else if *worker {
			vm = instance_types.CTWorkerInstance(num)
		} else {
			sklog.Fatal("Must specify exactly one of the builder flags or --master or --worker")
		}

		group.Go(vm.Name, func() error {
			if *create {
				return g.CreateAndSetup(ctx, vm, *ignoreExists)
			} else {
				return g.Delete(vm, *ignoreExists, *deleteDataDisk)
			}
		})
	}
	if err := group.Wait(); err != nil {
		sklog.Fatal(err)
	}
}
