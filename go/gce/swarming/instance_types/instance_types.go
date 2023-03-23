package instance_types

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/skerr"
	"gopkg.in/yaml.v2"
)

const (
	DEV_NAME_PREFIX      = "skia-d-"
	INTERNAL_NAME_PREFIX = "skia-i-"

	USER_CHROME_BOT = gce.USER_CHROME_BOT

	DEBIAN_SOURCE_IMAGE_EXTERNAL = "skia-swarming-base-v2020-03-31-000"
	DEBIAN_SOURCE_IMAGE_INTERNAL = "skia-swarming-base-v2020-03-31-000"
	WIN_SOURCE_IMAGE             = "projects/windows-cloud/global/images/windows-server-2019-dc-v20200114"

	INSTANCE_TYPE_CT            = "ct"
	INSTANCE_TYPE_LINUX_MICRO   = "linux-micro"
	INSTANCE_TYPE_LINUX_SMALL   = "linux-small"
	INSTANCE_TYPE_LINUX_MEDIUM  = "linux-medium"
	INSTANCE_TYPE_LINUX_LARGE   = "linux-large"
	INSTANCE_TYPE_LINUX_GPU     = "linux-gpu"
	INSTANCE_TYPE_LINUX_AMD     = "linux-amd"
	INSTANCE_TYPE_LINUX_SKYLAKE = "linux-skylake"
	INSTANCE_TYPE_WIN_MEDIUM    = "win-medium"
	INSTANCE_TYPE_WIN_LARGE     = "win-large"
)

var (
	// "Constants"
	SETUP_SCRIPT_LINUX_CT_PATH    = filepath.Join("go", "gce", "swarming", "setup-script-linux-ct.sh")
	SETUP_SCRIPT_LINUX_PATH       = filepath.Join("go", "gce", "swarming", "setup-script-linux.sh")
	SETUP_SCRIPT_WIN_ANSIBLE_PATH = filepath.Join("go", "gce", "swarming", "setup-win.ps1")
	NODE_SETUP_PATH               = filepath.Join("third_party", "node", "setup_6.x")

	externalNamePrefixRegexp = regexp.MustCompile("^skia-e-")
)

// Base configs for Swarming GCE instances.
func Swarming20180406(name string, machineType, serviceAccount, setupScript, nodeSetupScript, sourceImage string) *gce.Instance {
	vm := &gce.Instance{
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
		SetupScript:       setupScript,
		Scopes:            []string{auth.ScopeAllCloudAPIs, auth.ScopeUserinfoEmail, auth.ScopeFullControl},
		Tags:              []string{"use-swarming-auth"},
		User:              USER_CHROME_BOT,
	}
	if nodeSetupScript != "" {
		vm.Metadata["node-setup-script"] = nodeSetupScript
	}
	return vm
}

// Linux GCE instances.
func linuxSwarmingBot(num int, machineType, setupScript, nodeSetupScript string) *gce.Instance {
	return Swarming20180406(fmt.Sprintf("skia-e-gce-%03d", num), machineType, gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, setupScript, nodeSetupScript, DEBIAN_SOURCE_IMAGE_EXTERNAL)
}

// Micro Linux GCE instances.
func LinuxMicro(num int, setupScript, nodeSetupScript string) *gce.Instance {
	vm := linuxSwarmingBot(num, gce.MACHINE_TYPE_F1_MICRO, setupScript, nodeSetupScript)
	vm.DataDisks[0].SizeGb = 10
	return vm
}

// Small Linux GCE instances.
func LinuxSmall(num int, setupScript, nodeSetupScript string) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_HIGHMEM_2, setupScript, nodeSetupScript)
}

// Medium Linux GCE instances.
func LinuxMedium(num int, setupScript, nodeSetupScript string) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_STANDARD_16, setupScript, nodeSetupScript)
}

// Large Linux GCE instances.
func LinuxLarge(num int, setupScript, nodeSetupScript string) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_HIGHCPU_64, setupScript, nodeSetupScript)
}

// Linux GCE instances with GPUs.
func LinuxGpu(num int, setupScript, nodeSetupScript string) *gce.Instance {
	// Max 8 CPUs when using a GPU.
	vm := linuxSwarmingBot(num, gce.MACHINE_TYPE_STANDARD_8, setupScript, nodeSetupScript)
	vm.Gpu = true
	vm.MaintenancePolicy = gce.MAINTENANCE_POLICY_TERMINATE // Required for GPUs.
	return vm
}

// Linux GCE instances with AMD CPUs (skbug.com/10269).
func LinuxAmd(num int, setupScript, nodeSetupScript string) *gce.Instance {
	vm := linuxSwarmingBot(num, gce.MACHINE_TYPE_N2D_STANDARD_16, setupScript, nodeSetupScript)
	vm.MinCpuPlatform = gce.CPU_PLATFORM_AMD
	return vm
}

// Linux GCE instances with Skylake CPUs.
func LinuxSkylake(num int, setupScript, nodeSetupScript string) *gce.Instance {
	vm := LinuxMedium(num, setupScript, nodeSetupScript)
	vm.MinCpuPlatform = gce.CPU_PLATFORM_SKYLAKE
	return vm
}

// Internal instances.
func Internal(vm *gce.Instance) *gce.Instance {
	vm.Name = externalNamePrefixRegexp.ReplaceAllString(vm.Name, INTERNAL_NAME_PREFIX)
	for _, d := range append(vm.DataDisks, vm.BootDisk) {
		d.Name = externalNamePrefixRegexp.ReplaceAllString(d.Name, INTERNAL_NAME_PREFIX)
	}
	vm.ServiceAccount = gce.SERVICE_ACCOUNT_CHROME_SWARMING
	if vm.BootDisk.SourceImage == DEBIAN_SOURCE_IMAGE_EXTERNAL {
		vm.BootDisk.SourceImage = DEBIAN_SOURCE_IMAGE_INTERNAL
	}
	return vm
}

// Dev instances.
func Dev(vm *gce.Instance) *gce.Instance {
	vm.Name = externalNamePrefixRegexp.ReplaceAllString(vm.Name, DEV_NAME_PREFIX)
	for _, d := range append(vm.DataDisks, vm.BootDisk) {
		d.Name = externalNamePrefixRegexp.ReplaceAllString(d.Name, DEV_NAME_PREFIX)
	}
	return vm
}

// Skia CT bots.
func SkiaCT(num int, setupScript, nodeSetupScript string) *gce.Instance {
	vm := Swarming20180406(fmt.Sprintf("skia-ct-gce-%03d", num), gce.MACHINE_TYPE_STANDARD_16, gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, setupScript, nodeSetupScript, DEBIAN_SOURCE_IMAGE_EXTERNAL)
	vm.DataDisks[0].SizeGb = 3000
	// SkiaCT bots use a datadisk with a snapshot that is prepopulated with 1M SKPS.
	vm.DataDisks[0].SourceSnapshot = "skia-ct-skps-snapshot-3"
	return vm
}

// Configs for Windows GCE instances.
func AddWinConfigs(vm *gce.Instance, bootDiskType string) *gce.Instance {
	vm.BootDisk.SizeGb = 300
	vm.BootDisk.Type = bootDiskType
	vm.DataDisks = nil
	vm.Os = gce.OS_WINDOWS
	return vm
}

// Windows GCE instances.
func WinSwarmingBot(name, machineType, setupScript, bootDiskType string) *gce.Instance {
	vm := Swarming20180406(name, machineType, gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, setupScript, "", WIN_SOURCE_IMAGE)
	return AddWinConfigs(vm, bootDiskType)
}

// Medium Windows GCE instances.
func WinMedium(num int, setupScript string) *gce.Instance {
	return WinSwarmingBot(fmt.Sprintf("skia-e-gce-%03d", num), gce.MACHINE_TYPE_STANDARD_16, setupScript, gce.DISK_TYPE_PERSISTENT_SSD)
}

// Large Windows GCE instances.
func WinLarge(num int, setupScript string) *gce.Instance {
	return WinSwarmingBot(fmt.Sprintf("skia-e-gce-%03d", num), gce.MACHINE_TYPE_HIGHCPU_64, setupScript, gce.DISK_TYPE_PERSISTENT_SSD)
}

// Returns the contents of the VM setup script and NodeJS setup script necessary for CT machines,
// given a local checkout.
//
// Note that CT machines are not configured via Ansible at this time.
//
// TODO(lovisolo): Should we configure CT machines via Ansible as well?
func GetLinuxScriptsForCT(ctx context.Context, checkoutRoot, workdir string) (string, string, error) {
	setupScriptBytes, err := os.ReadFile(filepath.Join(checkoutRoot, SETUP_SCRIPT_LINUX_CT_PATH))
	if err != nil {
		return "", "", skerr.Wrap(err)
	}
	nodeSetupBytes, err := os.ReadFile(filepath.Join(checkoutRoot, NODE_SETUP_PATH))
	if err != nil {
		return "", "", skerr.Wrap(err)
	}
	return string(setupScriptBytes), string(nodeSetupBytes), nil
}

// Returns the contents of the setup script, given a local checkout.
func GetLinuxScript(checkoutRoot string) (string, error) {
	setupScriptBytes, err := os.ReadFile(filepath.Join(checkoutRoot, SETUP_SCRIPT_LINUX_PATH))
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return string(setupScriptBytes), nil
}

// Returns the setup script, given a local checkout. Writes the script into the given workdir.
func GetWindowsSetupScript(ctx context.Context, checkoutRoot, workdir string) (string, error) {
	chromeBotSkoloPassword, err := getChromeBotSkoloPassword(ctx, checkoutRoot)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	setupScriptTemplateBytes, err := ioutil.ReadFile(filepath.Join(checkoutRoot, SETUP_SCRIPT_WIN_ANSIBLE_PATH))
	if err != nil {
		return "", err
	}
	setupScript := strings.Replace(string(setupScriptTemplateBytes), "CHROME_BOT_PASSWORD", chromeBotSkoloPassword, -1)
	return setupScript, nil
}

// getChromeBotSkoloPassword retrieves the chrome-bot Skolo password from Berglas.
func getChromeBotSkoloPassword(ctx context.Context, checkoutRoot string) (string, error) {
	// Read secret using Berglas.
	berglasOutput := bytes.Buffer{}
	cmd := &exec.Command{
		Name:           "/bin/bash",
		Args:           []string{filepath.Join(checkoutRoot, "kube", "secrets", "get-secret.sh"), "etc", "ansible-secret-vars"},
		CombinedOutput: &berglasOutput,
	}
	if err := exec.Run(ctx, cmd); err != nil {
		return "", skerr.Wrap(err)
	}

	// Parse Berglas output.
	berglasSecret := struct {
		ApiVersion string `yaml:"apiVersion"`
		Data       struct {
			SecretsYml string `yaml:"secrets.yml"`
		} `yaml:"data"`
	}{}
	if err := yaml.Unmarshal(berglasOutput.Bytes(), &berglasSecret); err != nil {
		return "", skerr.Wrap(err)
	}

	// Decode and parse the secrets.yml file inside the Berglas secret.
	secretsYmlBytes, err := base64.StdEncoding.DecodeString(berglasSecret.Data.SecretsYml)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	secretsYml := struct {
		Secrets struct {
			SkoloPassword string `yaml:"skolo_password"`
		} `yaml:"secrets"`
	}{}
	if err := yaml.Unmarshal(secretsYmlBytes, &secretsYml); err != nil {
		return "", skerr.Wrap(err)
	}
	return secretsYml.Secrets.SkoloPassword, nil
}
