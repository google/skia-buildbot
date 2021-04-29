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
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags.
	instances       = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	windowsInstance = flag.Bool("windows", false, "Whether the created instances should be windows.")
	androidBuilder  = flag.Bool("android-builder", false, "Whether or not this is an android builder instance.")
	linuxBuilder    = flag.Bool("linux-builder", false, "Whether or not this is a linux builder instance.")
	windowsBuilder  = flag.Bool("windows-builder", false, "Whether or not this is a windows builder instance.")
	master          = flag.Bool(git.MasterBranch, false, "Whether or not this is a linux master instance.")
	worker          = flag.Bool("worker", false, "Whether or not this is a linux worker instance.")
	create          = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete          = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk  = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	ignoreExists    = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	workdir         = flag.String("workdir", ".", "Working directory.")

	// Scripts for creating Windows instances.
	winSetupScript     string
	winStartupScript   string
	winChromebotScript string
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
	if *windowsBuilder {
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

	if *windowsBuilder && !*windowsInstance {
		sklog.Fatal("--windows must be specified if --windows-builder is specified.")
	}

	ctx := context.Background()

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Set Windows scripts if we need to create Windows instances.
	if *windowsInstance {
		if err := setWindowsScripts(ctx, wdAbs); err != nil {
			sklog.Fatal(err)
		}
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
	group := util.NewNamedErrGroup()
	for _, num := range instanceNums {
		var vm *gce.Instance
		if *androidBuilder {
			vm = instance_types.CTAndroidBuilderInstance(num)
		} else if *linuxBuilder {
			vm = instance_types.CTLinuxBuilderInstance(num)
		} else if *windowsInstance {
			if *windowsBuilder {
				vm = instance_types.CTWindowsBuilderInstance(num, winSetupScript, winStartupScript, winChromebotScript)
			} else {
				vm = instance_types.CTWindowsInstance(num, winSetupScript, winStartupScript, winChromebotScript)
			}
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

func setWindowsScripts(ctx context.Context, workDir string) error {
	_, filename, _, _ := runtime.Caller(0)
	checkoutRoot := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
	var err error
	winSetupScript, winStartupScript, winChromebotScript, err = skia_instance_types.GetWindowsScripts(ctx, checkoutRoot, workDir)
	if err != nil {
		return err
	}

	// Set .netrc contents in the setup script.
	// TODO(rmistry): This should NOT be required. Investigate how to give the
	// service account all required permissions.
	netrcContents, err := exec.RunCwd(ctx, ".", "gsutil", "cat", instance_types.GS_URL_NETRC)
	if err != nil {
		return err
	}
	input, err := ioutil.ReadFile(winSetupScript)
	if err != nil {
		return err
	}
	output := bytes.Replace(input, []byte("INSERTFILE(/tmp/.netrc)"), []byte(netrcContents), -1)
	if err = ioutil.WriteFile(winSetupScript, output, 0666); err != nil {
		return err
	}

	// Set .boto contents in the setup script. See skbug.com/9392 for context.
	botoContents, err := exec.RunCwd(ctx, ".", "gsutil", "cat", instance_types.GS_URL_BOTO)
	if err != nil {
		return err
	}
	output = bytes.Replace(input, []byte("INSERTFILE(/tmp/.boto)"), []byte(botoContents), -1)
	if err = ioutil.WriteFile(winSetupScript, output, 0666); err != nil {
		return err
	}
	return nil
}
