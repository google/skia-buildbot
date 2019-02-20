package main

/*
   Program for automating creation and setup of Swarming bot VMs.
*/

import (
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/ct/instance_types"
	skia_instance_types "go.skia.org/infra/go/gce/swarming/instance_types"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags.
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	androidBuilder = flag.Bool("android-builder", false, "Whether or not this is an android builder instance.")
	linuxBuilder   = flag.Bool("linux-builder", false, "Whether or not this is a linux builder instance.")
	windowsBuilder = flag.Bool("windows-builder", false, "Whether or not this is a windows builder instance.")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	workdir        = flag.String("workdir", ".", "Working directory.")
)

func main() {
	common.Init()

	// Validation.
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
	}

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
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
		} else if *windowsBuilder {
			_, filename, _, _ := runtime.Caller(0)
			checkoutRoot := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
			setupScript, startupScript, chromebotScript, err := skia_instance_types.GetWindowsScripts(ctx, checkoutRoot, wdAbs)
			if err != nil {
				sklog.Fatal(err)
			}

			// Set .netrc contents in the setup script.
			// TODO(rmistry): This should NOT be required. Investigate how to give the
			// service account all required permissions.
			netrcContents, err := exec.RunCwd(ctx, ".", "gsutil", "cat", instance_types.GS_URL_NETRC)
			if err != nil {
				sklog.Fatal(err)
			}
			input, err := ioutil.ReadFile(setupScript)
			if err != nil {
				sklog.Fatal(err)
			}
			output := bytes.Replace(input, []byte("INSERTFILE(/tmp/.netrc)"), []byte(netrcContents), -1)
			if err = ioutil.WriteFile(setupScript, output, 0666); err != nil {
				sklog.Fatal(err)
			}

			vm = instance_types.CTWindowsBuilderInstance(num, setupScript, startupScript, chromebotScript)
		} else {
			vm = instance_types.CTInstance(num)
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
