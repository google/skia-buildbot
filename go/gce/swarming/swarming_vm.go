package main

/*
   Program for automating creation and setup of Swarming bot VMs.

   Bot numbers should be assigned as follows:
     1-99 (skia-gce-0..): Temporary or experimental bots.
     100-499 (skia-gce-[1234]..): Linux
       100-199: Haswell, Debian9, n1-highmem-2
       200-299: Haswell, Debian9, n1-standard-16
       300-399: Haswell, Debian9, n1-highcpu-64
       400-499: Skylake, Debian9, n1-standard-16
     500-699 (skia-gce-[56]..): Windows
       500-599: Haswell, Win2016, n1-highmem-2
       600-699: Haswell, Win2016, n1-highcpu-64
     700-999: unassigned
*/

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/swarming/instance_types"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	USER_CHROME_BOT = "chrome-bot"

	OS_DEBIAN_9 = "Debian9"
	OS_WIN_2016 = "Win2016"

	DEBIAN_SOURCE_IMAGE_EXTERNAL = "skia-swarming-base-v2018-04-06-000"
	DEBIAN_SOURCE_IMAGE_INTERNAL = "skia-swarming-base-v2018-04-09-000"
	WIN_SOURCE_IMAGE             = "projects/windows-cloud/global/images/windows-server-2016-dc-v20190108"
	INSTANCE_TYPE_CT             = "ct"
	INSTANCE_TYPE_LINUX_SMALL    = "linux-small"
	INSTANCE_TYPE_LINUX_MEDIUM   = "linux-medium"
	INSTANCE_TYPE_LINUX_LARGE    = "linux-large"
	INSTANCE_TYPE_LINUX_GPU      = "linux-gpu"
	INSTANCE_TYPE_LINUX_SKYLAKE  = "linux-skylake"
	INSTANCE_TYPE_WIN_MEDIUM     = "win-medium"
	INSTANCE_TYPE_WIN_LARGE      = "win-large"
)

var (
	VALID_INSTANCE_TYPES = []string{
		INSTANCE_TYPE_CT,
		INSTANCE_TYPE_LINUX_SMALL,
		INSTANCE_TYPE_LINUX_MEDIUM,
		INSTANCE_TYPE_LINUX_LARGE,
		INSTANCE_TYPE_LINUX_GPU,
		INSTANCE_TYPE_LINUX_SKYLAKE,
		INSTANCE_TYPE_WIN_MEDIUM,
		INSTANCE_TYPE_WIN_LARGE,
	}
	WIN_INSTANCE_TYPES = []string{
		INSTANCE_TYPE_WIN_MEDIUM,
		INSTANCE_TYPE_WIN_LARGE,
	}
)

var (
	// Flags.
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	dumpJson       = flag.Bool("dump-json", false, "Dump out JSON for each of the instances to create/delete and exit without changing anything.")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	instanceType   = flag.String("type", "", fmt.Sprintf("Type of instance; one of: %v", VALID_INSTANCE_TYPES))
	internal       = flag.Bool("internal", false, "Whether or not the bots are internal.")
	workdir        = flag.String("workdir", ".", "Working directory.")
)

// Base configs for Swarming GCE instances.
func Swarming20180406(name, serviceAccount, sourceImage string) *gce.Instance {
	return &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        name,
			SizeGb:      15,
			SourceImage: sourceImage,
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisks: []*gce.Disk{{
			Name:      fmt.Sprintf("%s-data", name),
			SizeGb:    int64(*dataDiskSize),
			Type:      gce.DISK_TYPE_PERSISTENT_STANDARD,
			MountPath: gce.DISK_MOUNT_PATH_DEFAULT,
		}},
		Gpu:               *gpu,
		GSDownloads:       []*gce.GSDownload{},
		MachineType:       *machineType,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Os:                gce.OS_LINUX,
		ServiceAccount:    serviceAccount,
		Scopes: []string{
			auth.SCOPE_FULL_CONTROL,
			auth.SCOPE_USERINFO_EMAIL,
			auth.SCOPE_PUBSUB,
		},
		Tags: []string{"use-swarming-auth"},
		User: USER_CHROME_BOT,
	}
}

// Configs for Linux GCE instances.
func AddLinuxConfigs(vm *gce.Instance) *gce.Instance {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script-linux.sh")
	return vm
}

// Linux GCE instances.
func LinuxSwarmingBot(num int) *gce.Instance {
	return AddLinuxConfigs(Swarming20180406(fmt.Sprintf("skia-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, DEBIAN_SOURCE_IMAGE_EXTERNAL))
}

// Internal Linux GCE instances.
func InternalLinuxSwarmingBot(num int) *gce.Instance {
	return AddLinuxConfigs(Swarming20180406(fmt.Sprintf("skia-i-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROME_SWARMING, DEBIAN_SOURCE_IMAGE_INTERNAL))
}

// Dev Linux GCE instances.
func DevLinuxSwarmingBot(num int) *gce.Instance {
	return AddLinuxConfigs(Swarming20180406(fmt.Sprintf("skia-d-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, DEBIAN_SOURCE_IMAGE_EXTERNAL))
}

// Skia CT bots.
func SkiaCTBot(num int) *gce.Instance {
	vm := AddLinuxConfigs(Swarming20180406(fmt.Sprintf("skia-ct-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, DEBIAN_SOURCE_IMAGE_EXTERNAL))
	vm.DataDisks[0].SizeGb = 3000
	// SkiaCT bots use a datadisk with a snapshot that is prepopulated with 1M SKPS.
	vm.DataDisks[0].SourceSnapshot = "skia-ct-skps-snapshot-3"
	return vm
}

// Configs for Windows GCE instances.
func AddWinConfigs(vm *gce.Instance, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm.BootDisk.SizeGb = 300
	vm.BootDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.DataDisks = nil
	// Most of the Windows setup, including the gitconfig/netrc, occurs in
	// the setup and startup scripts, which also install and schedule the
	// chrome-bot scheduled task script.
	vm.Metadata["chromebot-schtask-ps1"] = chromebotScript
	vm.Os = gce.OS_WINDOWS
	vm.SetupScript = setupScriptPath
	vm.StartupScript = startupScriptPath
	return vm
}

// Windows GCE instances.
func WinSwarmingBot(num int, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm := Swarming20180406(fmt.Sprintf("skia-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, WIN_SOURCE_IMAGE)
	return AddWinConfigs(vm, setupScriptPath, startupScriptPath, chromebotScript)
}

// Internal Windows GCE instances.
func InternalWinSwarmingBot(num int, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm := Swarming20180406(fmt.Sprintf("skia-i-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROME_SWARMING, WIN_SOURCE_IMAGE)
	return AddWinConfigs(vm, setupScriptPath, startupScriptPath, chromebotScript)
}

// Dev Windows GCE instances.
func DevWinSwarmingBot(num int, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm := Swarming20180406(fmt.Sprintf("skia-d-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, WIN_SOURCE_IMAGE)
	return AddWinConfigs(vm, setupScriptPath, startupScriptPath, chromebotScript)
}

// GCE instances with GPUs.
func AddGpuConfigs(vm *gce.Instance) *gce.Instance {
	vm.Gpu = true
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_8            // Max 8 CPUs when using a GPU.
	vm.MaintenancePolicy = gce.MAINTENANCE_POLICY_TERMINATE // Required for GPUs.
	return vm
}

// GCE instances with Skylake CPUs.
func AddSkylakeConfigs(vm *gce.Instance) *gce.Instance {
	vm.MinCpuPlatform = gce.CPU_PLATFORM_SKYLAKE
	return vm
}

// Returns the setup, startup, and chrome-bot scripts.
func getWindowsScripts(ctx context.Context, workdir string) (string, string, string, error) {
	pw, err := exec.RunCwd(ctx, ".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/win-chrome-bot.txt")
	if err != nil {
		return "", "", "", err
	}
	pw = strings.TrimSpace(pw)

	_, filename, _, _ := runtime.Caller(0)
	repoRoot := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
	setupBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "win_setup.ps1"))
	if err != nil {
		return "", "", "", err
	}
	setupScript := strings.Replace(string(setupBytes), "CHROME_BOT_PASSWORD", pw, -1)

	setupPath := path.Join(workdir, "setup-script.ps1")
	if err := ioutil.WriteFile(setupPath, []byte(setupScript), os.ModePerm); err != nil {
		return "", "", "", err
	}

	startupBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "win_startup.ps1"))
	if err != nil {
		return "", "", "", err
	}
	startupScript := strings.Replace(string(startupBytes), "CHROME_BOT_PASSWORD", pw, -1)
	startupPath := path.Join(workdir, "startup-script.ps1")
	if err := ioutil.WriteFile(startupPath, []byte(startupScript), os.ModePerm); err != nil {
		return "", "", "", err
	}

	// Return the chrome-bot script itself, not its path.
	chromebotBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "chromebot-schtask.ps1"))
	if err != nil {
		return "", "", "", err
	}
	chromebotScript := util.ToDos(string(chromebotBytes))
	return setupPath, startupPath, chromebotScript, nil
}

func main() {
	common.Init()

	// Validation.
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
	}
	if !util.In(*instanceType, VALID_INSTANCE_TYPES) {
		sklog.Fatalf("--type must be one of %v", VALID_INSTANCE_TYPES)
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

	// Read the various scripts.
	_, filename, _, _ := runtime.Caller(0)
	checkoutRoot := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
	ctx := context.Background()
	var setupScript, startupScript, chromebotScript string
	if util.In(*instanceType, WIN_INSTANCE_TYPES) {
		setupScript, startupScript, chromebotScript, err = instance_types.GetWindowsScripts(ctx, checkoutRoot, wdAbs)
	} else {
		setupScript, err = instance_types.GetLinuxScripts(ctx, checkoutRoot, wdAbs)
	}
	if err != nil {
		sklog.Fatal(err)
	}

	zone := gce.ZONE_DEFAULT
	project := gce.PROJECT_ID_SWARMING
	var getInstance func(int) *gce.Instance
	switch *instanceType {
	case INSTANCE_TYPE_CT:
		getInstance = func(num int) *gce.Instance { return instance_types.SkiaCT(num, setupScript) }
	case INSTANCE_TYPE_LINUX_SMALL:
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxSmall(num, setupScript) }
	case INSTANCE_TYPE_LINUX_MEDIUM:
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxMedium(num, setupScript) }
	case INSTANCE_TYPE_LINUX_LARGE:
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxLarge(num, setupScript) }
	case INSTANCE_TYPE_LINUX_GPU:
		zone = gce.ZONE_GPU
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxGpu(num, setupScript) }
	case INSTANCE_TYPE_LINUX_SKYLAKE:
		zone = gce.ZONE_SKYLAKE
		getInstance = func(num int) *gce.Instance { return instance_types.LinuxSkylake(num, setupScript) }
	case INSTANCE_TYPE_WIN_MEDIUM:
		getInstance = func(num int) *gce.Instance {
			return instance_types.WinMedium(num, setupScript, startupScript, chromebotScript)
		}
	case INSTANCE_TYPE_WIN_LARGE:
		getInstance = func(num int) *gce.Instance {
			return instance_types.WinLarge(num, setupScript, startupScript, chromebotScript)
		}
	}
	if getInstance == nil {
		sklog.Fatalf("Could not find matching instance type for --type %s", *instanceType)
	}
	if *internal {
		project = gce.PROJECT_ID_SERVER
		getInstanceInner := getInstance
		getInstance = func(num int) *gce.Instance {
			return instance_types.Internal(getInstanceInner(num))
		}
	}

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
					if err := g.WaitForLogMessage(vm, "*** Start Swarming. ***", 5*time.Minute); err != nil {
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
