package main

/*
   Program for automating creation and setup of Swarming bot VMs.
*/

import (
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
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GS_URL_GITCONFIG      = "gs://skia-buildbots/artifacts/bots/.gitconfig"
	GS_URL_NETRC_EXTERNAL = "gs://skia-buildbots/artifacts/bots/.netrc_bots"
	GS_URL_NETRC_INTERNAL = "gs://skia-buildbots/artifacts/bots/.netrc_bots-internal"

	USER_CHROME_BOT = "chrome-bot"

	OS_DEBIAN_9 = "Debian9"
	OS_WIN_2K8  = "Win2k8"
	OS_WIN_2016 = "Win2016"
)

var (
	VALID_OS = []string{OS_DEBIAN_9, OS_WIN_2K8, OS_WIN_2016}
)

var (
	// Flags.
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	ct             = flag.Bool("skia-ct", false, "If true, this is a bot in the SkiaCT pool.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	gpu            = flag.Bool("gpu", false, "Whether or not to add an NVIDIA Tesla k80 GPU on the instance(s)")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	internal       = flag.Bool("internal", false, "Whether or not the bots are internal.")
	opsys          = flag.String("os", OS_DEBIAN_9, fmt.Sprintf("OS identifier; one of %s", strings.Join(VALID_OS, ", ")))
	skylake        = flag.Bool("skylake", false, "Whether or not the instance(s) should use Intel Skylake CPUs.")
	workdir        = flag.String("workdir", ".", "Working directory.")
)

// Base configs for Swarming GCE instances.
func Swarming20171114(name, serviceAccount string) *gce.Instance {
	return &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        name,
			SourceImage: "skia-swarming-base-v2017-11-14-000",
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisks: []*gce.Disk{{
			Name:      fmt.Sprintf("%s-data", name),
			SizeGb:    300,
			Type:      gce.DISK_TYPE_PERSISTENT_STANDARD,
			MountPath: gce.DISK_MOUNT_PATH_DEFAULT,
		}},
		Gpu:               *gpu,
		GSDownloads:       []*gce.GSDownload{},
		MachineType:       gce.MACHINE_TYPE_STANDARD_16,
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
func AddLinuxConfigs(vm *gce.Instance, netrcUrl string) *gce.Instance {
	vm.GSDownloads = append(vm.GSDownloads, &gce.GSDownload{
		Source: GS_URL_GITCONFIG,
		Dest:   "/home/chrome-bot/.gitconfig",
	}, &gce.GSDownload{
		Source: netrcUrl,
		Dest:   "/home/chrome-bot/.netrc",
		Mode:   "600",
	})

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script-linux.sh")
	return vm
}

// Linux GCE instances.
func LinuxSwarmingBot(num int) *gce.Instance {
	return AddLinuxConfigs(Swarming20171114(fmt.Sprintf("skia-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROMIUM_SWARM), GS_URL_NETRC_EXTERNAL)
}

// Internal Linux GCE instances.
func InternalLinuxSwarmingBot(num int) *gce.Instance {
	return AddLinuxConfigs(Swarming20171114(fmt.Sprintf("skia-i-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROME_SWARMING), GS_URL_NETRC_INTERNAL)
}

// Skia CT bots.
func SkiaCTBot(num int) *gce.Instance {
	vm := AddLinuxConfigs(Swarming20171114(fmt.Sprintf("skia-ct-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROMIUM_SWARM), GS_URL_NETRC_EXTERNAL)
	vm.DataDisks[0].SizeGb = 3000
	// SkiaCT bots use a datadisk with a snapshot that is prepopulated with 1M SKPS.
	vm.DataDisks[0].SourceSnapshot = "skia-ct-skps-snapshot-2"
	return vm
}

// Configs for Windows GCE instances.
func AddWinConfigs(vm *gce.Instance, opsys, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm.BootDisk.SizeGb = 300
	switch opsys {
	case OS_WIN_2K8:
		vm.BootDisk.SourceImage = "projects/google.com:windows-internal/global/images/windows-server-2008-r2-ent-internal-v20150310"
	case OS_WIN_2016:
		vm.BootDisk.SourceImage = "projects/google.com:windows-internal/global/images/windows-server-2016-dc-internal-v20171010"
	default:
		// Shouldn't happen.
		sklog.Fatalf("Invalid os %q", opsys)
	}
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
func WinSwarmingBot(num int, opsys, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm := Swarming20171114(fmt.Sprintf("skia-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROMIUM_SWARM)
	return AddWinConfigs(vm, opsys, setupScriptPath, startupScriptPath, chromebotScript)
}

// Internal Windows GCE instances.
func InternalWinSwarmingBot(num int, opsys, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm := Swarming20171114(fmt.Sprintf("skia-i-gce-%03d", num), gce.SERVICE_ACCOUNT_CHROME_SWARMING)
	return AddWinConfigs(vm, opsys, setupScriptPath, startupScriptPath, chromebotScript)
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
func getWindowsScripts(workdir string) (string, string, string, error) {
	pw, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/win-chrome-bot.txt")
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

	netrcContents, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/.netrc_bots")
	if err != nil {
		return "", "", "", err
	}
	setupScript = strings.Replace(setupScript, "INSERTFILE(/tmp/.netrc)", string(netrcContents), -1)

	gitconfigContents, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/.gitconfig")
	if err != nil {
		return "", "", "", err
	}
	setupScript = strings.Replace(setupScript, "INSERTFILE(/tmp/.gitconfig)", string(gitconfigContents), -1)
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
	defer common.LogPanic()

	// Validation.
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
	}

	if !util.In(*opsys, VALID_OS) {
		sklog.Fatalf("Unknown --os %q", *opsys)
	}
	windows := strings.HasPrefix(*opsys, "Win")

	if *ct && windows {
		sklog.Fatalf("--skia-ct does not support %q.", *opsys)
	}
	if *skylake && *gpu {
		sklog.Fatal("--skylake and --gpu are mutually exclusive.")
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
	zone := gce.ZONE_DEFAULT
	if *gpu {
		zone = gce.ZONE_GPU
	} else if *skylake {
		zone = gce.ZONE_SKYLAKE
	}
	g, err := gce.NewGCloud(zone, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}

	// Read the various Windows scripts.
	var setupScript, startupScript, chromebotScript string
	if windows {
		setupScript, startupScript, chromebotScript, err = getWindowsScripts(wdAbs)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Perform the requested operation.
	group := util.NewNamedErrGroup()
	for _, num := range instanceNums {
		var vm *gce.Instance
		if *ct {
			vm = SkiaCTBot(num)
		} else if windows {
			if *internal {
				vm = InternalWinSwarmingBot(num, *opsys, setupScript, startupScript, chromebotScript)
			} else {
				vm = WinSwarmingBot(num, *opsys, setupScript, startupScript, chromebotScript)
			}
		} else {
			if *internal {
				vm = InternalLinuxSwarmingBot(num)
			} else {
				vm = LinuxSwarmingBot(num)
			}
		}
		if *gpu {
			AddGpuConfigs(vm)
		} else if *skylake {
			AddSkylakeConfigs(vm)
		}

		group.Go(vm.Name, func() error {
			if *create {
				if err := g.CreateAndSetup(vm, *ignoreExists); err != nil {
					return err
				}

				if windows {
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
