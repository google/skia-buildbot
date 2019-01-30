package instance_types

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/util"
)

const (
	INTERNAL_NAME_PREFIX = "skia-i-"

	USER_CHROME_BOT = "chrome-bot"

	OS_DEBIAN_9 = "Debian9"
	OS_WIN_2016 = "Win2016"

	DEBIAN_SOURCE_IMAGE_EXTERNAL = "skia-swarming-base-v2018-04-06-000"
	DEBIAN_SOURCE_IMAGE_INTERNAL = "skia-swarming-base-v2018-04-09-000"
	WIN_SOURCE_IMAGE             = "projects/windows-cloud/global/images/windows-server-2016-dc-v20190108"
)

var (
	// "Constants"
	SETUP_SCRIPT_LINUX_PATH    = filepath.Join("go", "gce", "swarming", "setup-script-linux.sh")
	SETUP_SCRIPT_WIN_PATH      = filepath.Join("scripts", "win_setup.ps1")
	STARTUP_SCRIPT_WIN_PATH    = filepath.Join("scripts", "win_startup.ps1")
	CHROME_BOT_SCRIPT_WIN_PATH = filepath.Join("scripts", "chromebot-schtask.ps1")

	externalNamePrefixRegexp = regexp.MustCompile("^skia-")
)

// Base configs for Swarming GCE instances.
func Swarming20180406(name string, machineType, serviceAccount, setupScriptPath, sourceImage string) *gce.Instance {
	return &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        name,
			SizeGb:      15,
			SourceImage: sourceImage,
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisks: []*gce.Disk{{
			Name:      fmt.Sprintf("%s-data", name),
			SizeGb:    300,
			Type:      gce.DISK_TYPE_PERSISTENT_STANDARD,
			MountPath: gce.DISK_MOUNT_PATH_DEFAULT,
		}},
		GSDownloads:       []*gce.GSDownload{},
		MachineType:       machineType,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Os:                gce.OS_LINUX,
		ServiceAccount:    serviceAccount,
		SetupScript:       setupScriptPath,
		Scopes: []string{
			auth.SCOPE_FULL_CONTROL,
			auth.SCOPE_USERINFO_EMAIL,
			auth.SCOPE_PUBSUB,
		},
		Tags: []string{"use-swarming-auth"},
		User: USER_CHROME_BOT,
	}
}

// Linux GCE instances.
func linuxSwarmingBot(num int, machineType, setupScriptPath string) *gce.Instance {
	return Swarming20180406(fmt.Sprintf("skia-gce-%03d", num), machineType, gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, setupScriptPath, DEBIAN_SOURCE_IMAGE_EXTERNAL)
}

// Micro Linux GCE instances.
func LinuxMicro(num int, setupScriptPath string) *gce.Instance {
	vm := linuxSwarmingBot(num, gce.MACHINE_TYPE_F1_MICRO, setupScriptPath)
	vm.DataDisks[0].SizeGb = 10
	return vm
}

// Small Linux GCE instances.
func LinuxSmall(num int, setupScriptPath string) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_HIGHMEM_2, setupScriptPath)
}

// Medium Linux GCE instances.
func LinuxMedium(num int, setupScriptPath string) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_STANDARD_16, setupScriptPath)
}

// Large Linux GCE instances.
func LinuxLarge(num int, setupScriptPath string) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_HIGHCPU_64, setupScriptPath)
}

// Linux GCE instances with GPUs.
func LinuxGpu(num int, setupScriptPath string) *gce.Instance {
	// Max 8 CPUs when using a GPU.
	vm := linuxSwarmingBot(num, gce.MACHINE_TYPE_STANDARD_8, setupScriptPath)
	vm.Gpu = true
	vm.MaintenancePolicy = gce.MAINTENANCE_POLICY_TERMINATE // Required for GPUs.
	return vm
}

// Linux GCE instances with Skylake CPUs.
func LinuxSkylake(num int, setupScriptPath string) *gce.Instance {
	vm := LinuxMedium(num, setupScriptPath)
	vm.MinCpuPlatform = gce.CPU_PLATFORM_SKYLAKE
	return vm
}

// Internal instances.
func Internal(vm *gce.Instance) *gce.Instance {
	vm.Name = externalNamePrefixRegexp.ReplaceAllString(vm.Name, INTERNAL_NAME_PREFIX)
	vm.ServiceAccount = gce.SERVICE_ACCOUNT_CHROME_SWARMING
	if vm.BootDisk.SourceImage == DEBIAN_SOURCE_IMAGE_EXTERNAL {
		vm.BootDisk.SourceImage = DEBIAN_SOURCE_IMAGE_INTERNAL
	}
	return vm
}

// Skia CT bots.
func SkiaCT(num int, setupScriptPath string) *gce.Instance {
	vm := Swarming20180406(fmt.Sprintf("skia-ct-gce-%03d", num), gce.MACHINE_TYPE_STANDARD_16, gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, setupScriptPath, DEBIAN_SOURCE_IMAGE_EXTERNAL)
	vm.DataDisks[0].SizeGb = 3000
	// SkiaCT bots use a datadisk with a snapshot that is prepopulated with 1M SKPS.
	vm.DataDisks[0].SourceSnapshot = "skia-ct-skps-snapshot-3"
	return vm
}

// Configs for Windows GCE instances.
func addWinConfigs(vm *gce.Instance, startupScriptPath, chromebotScript string) *gce.Instance {
	vm.BootDisk.SizeGb = 300
	vm.BootDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.DataDisks = nil
	// Most of the Windows setup, including the gitconfig/netrc, occurs in
	// the setup and startup scripts, which also install and schedule the
	// chrome-bot scheduled task script.
	vm.Metadata["chromebot-schtask-ps1"] = chromebotScript
	vm.Os = gce.OS_WINDOWS
	vm.StartupScript = startupScriptPath
	return vm
}

// Windows GCE instances.
func winSwarmingBot(num int, machineType, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm := Swarming20180406(fmt.Sprintf("skia-gce-%03d", num), machineType, gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, setupScriptPath, WIN_SOURCE_IMAGE)
	return addWinConfigs(vm, startupScriptPath, chromebotScript)
}

// Medium Windows GCE instances.
func WinMedium(num int, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	return winSwarmingBot(num, gce.MACHINE_TYPE_STANDARD_16, setupScriptPath, startupScriptPath, chromebotScript)
}

// Large Windows GCE instances.
func WinLarge(num int, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	return winSwarmingBot(num, gce.MACHINE_TYPE_HIGHCPU_64, setupScriptPath, startupScriptPath, chromebotScript)
}

// Returns the path to the setup script, given a local checkout.
func GetLinuxScripts(ctx context.Context, checkoutRoot, workdir string) (string, error) {
	return filepath.Join(checkoutRoot, SETUP_SCRIPT_LINUX_PATH), nil
}

// Returns the setup, startup, and chrome-bot scripts, given a local checkout.
// Writes the scripts into the given workdir.
func GetWindowsScripts(ctx context.Context, checkoutRoot, workdir string) (string, string, string, error) {
	pw, err := exec.RunCwd(ctx, ".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/win-chrome-bot.txt")
	if err != nil {
		return "", "", "", err
	}
	pw = strings.TrimSpace(pw)

	setupBytes, err := ioutil.ReadFile(filepath.Join(checkoutRoot, SETUP_SCRIPT_WIN_PATH))
	if err != nil {
		return "", "", "", err
	}
	setupScript := strings.Replace(string(setupBytes), "CHROME_BOT_PASSWORD", pw, -1)

	setupPath := filepath.Join(workdir, "setup-script.ps1")
	if err := ioutil.WriteFile(setupPath, []byte(setupScript), os.ModePerm); err != nil {
		return "", "", "", err
	}

	startupBytes, err := ioutil.ReadFile(filepath.Join(checkoutRoot, STARTUP_SCRIPT_WIN_PATH))
	if err != nil {
		return "", "", "", err
	}
	startupScript := strings.Replace(string(startupBytes), "CHROME_BOT_PASSWORD", pw, -1)
	startupPath := filepath.Join(workdir, "startup-script.ps1")
	if err := ioutil.WriteFile(startupPath, []byte(startupScript), os.ModePerm); err != nil {
		return "", "", "", err
	}

	// Return the chrome-bot script itself, not its path.
	chromebotBytes, err := ioutil.ReadFile(filepath.Join(checkoutRoot, CHROME_BOT_SCRIPT_WIN_PATH))
	if err != nil {
		return "", "", "", err
	}
	chromebotScript := util.ToDos(string(chromebotBytes))
	return setupPath, startupPath, chromebotScript, nil
}
