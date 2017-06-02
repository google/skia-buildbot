package main

/*
   Program for automating creation and setup of Swarming bot VMs.
*/

import (
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GS_URL_GITCONFIG = "gs://skia-buildbots/artifacts/bots/.gitconfig"
	GS_URL_NETRC     = "gs://skia-buildbots/artifacts/bots/.netrc"

	IP_ADDRESS_TMPL = "104.154.123.%d"
	USER_CHROME_BOT = "chrome-bot"
)

var (
	// Flags.
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	builder        = flag.Bool("builder", false, "Whether or not this is a builder instance.")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	workdir        = flag.String("workdir", ".", "Working directory.")
)

// Base config for CT GCE instances.
func CT20170602(name, ipAddress string) *gce.Instance {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	return &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        name,
			SourceImage: "skia-swarming-v3",
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisk: &gce.Disk{
			Name:   fmt.Sprintf("%s-data", name),
			SizeGb: 300,
			Type:   gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		ExternalIpAddress: ipAddress,
		GSDownloads: map[string]string{
			"/home/chrome-bot/.gitconfig": GS_URL_GITCONFIG,
			"/home/chrome-bot/.netrc":     GS_URL_NETRC,
		},
		MachineType:       gce.MACHINE_TYPE_HIGHMEM_2,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Os:                gce.OS_LINUX,
		Scopes: []string{
			auth.SCOPE_FULL_CONTROL,
		},
		SetupScript: path.Join(dir, "setup-script.sh"),
		Tags:        []string{"http-server", "https-server"},
		User:        USER_CHROME_BOT,
	}
}

// CT GCE instances.
func CTInstance(num int, ipAddress string) *gce.Instance {
	return CT20170602(fmt.Sprintf("ct-vm-%03d", num), ipAddress)
}

// CT Builder GCE instances.
func CTBuilderInstance(num int, ipAddress string) *gce.Instance {
	vm := CT20170602(fmt.Sprintf("ct-vm-%03d", num), ipAddress)
	vm.MachineType = "custom-32-70400"
	return vm
}

func main() {
	common.Init()
	defer common.LogPanic()

	// Validation.
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
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

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the GCloud object.
	g, err := gce.NewGCloud(gce.ZONE_CT, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}

	// Perform the requested operation.
	group := util.NewNamedErrGroup()
	for _, num := range instanceNums {
		var vm *gce.Instance
		ipAddr := fmt.Sprintf(IP_ADDRESS_TMPL, num)
		if *builder {
			vm = CTBuilderInstance(num, ipAddr)
		} else {
			vm = CTInstance(num, ipAddr)
		}

		group.Go(vm.Name, func() error {
			if *create {
				return g.CreateAndSetup(vm, *ignoreExists, *workdir)
			} else {
				return g.Delete(vm, *ignoreExists, *deleteDataDisk)
			}
		})
	}
	if err := group.Wait(); err != nil {
		sklog.Fatal(err)
	}
}
