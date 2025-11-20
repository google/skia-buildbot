package instance_types

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/skerr"
	"gopkg.in/yaml.v2"

	_ "embed"
)

const (
	DEV_NAME_PREFIX      = "skia-d-"
	INTERNAL_NAME_PREFIX = "skia-i-"

	USER_CHROME_BOT = gce.USER_CHROME_BOT

	DEBIAN_SOURCE_IMAGE_EXTERNAL = "skia-swarming-base-v2020-03-31-000"
	DEBIAN_SOURCE_IMAGE_INTERNAL = "skia-swarming-base-v2020-03-31-000"
	WIN_SOURCE_IMAGE             = "projects/windows-cloud/global/images/windows-server-2019-dc-v20200114"

	INSTANCE_TYPE_LINUX_MICRO   = "linux-micro"
	INSTANCE_TYPE_LINUX_SMALL   = "linux-small"
	INSTANCE_TYPE_LINUX_MEDIUM  = "linux-medium"
	INSTANCE_TYPE_LINUX_LARGE   = "linux-large"
	INSTANCE_TYPE_LINUX_AMD     = "linux-amd"
	INSTANCE_TYPE_LINUX_SKYLAKE = "linux-skylake"
	INSTANCE_TYPE_WIN_MEDIUM    = "win-medium"
	INSTANCE_TYPE_WIN_LARGE     = "win-large"
)

var (
	//go:embed setup-script-linux.sh
	setupScriptLinuxSH string

	//go:embed setup-script-linux-ct.sh
	setupScriptLinuxCTSH string

	//go:embed setup-win.ps1
	setupWinPS1 string

	//go:embed third_party/node/setup_6.x
	nodeSetup6xScript string

	externalNamePrefixRegexp = regexp.MustCompile("^skia-e-")
)

// Base configs for Swarming GCE instances.
func swarming20180406(name string, machineType, serviceAccount, setupScript, sourceImage string) *gce.Instance {
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
		SetupScript:       setupScript,
		Scopes:            []string{auth.ScopeAllCloudAPIs, auth.ScopeUserinfoEmail, auth.ScopeFullControl},
		Tags:              []string{"use-swarming-auth"},
		User:              USER_CHROME_BOT,
	}
}

// Linux GCE instances.
func linuxSwarmingBot(num int, machineType string) *gce.Instance {
	return swarming20180406(fmt.Sprintf("skia-e-gce-%03d", num), machineType, gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, setupScriptLinuxSH, DEBIAN_SOURCE_IMAGE_EXTERNAL)
}

// Micro Linux GCE instances.
func LinuxMicro(num int) *gce.Instance {
	vm := linuxSwarmingBot(num, gce.MACHINE_TYPE_F1_MICRO)
	vm.DataDisks[0].SizeGb = 10
	return vm
}

// Small Linux GCE instances.
func LinuxSmall(num int) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_HIGHMEM_2)
}

// Medium Linux GCE instances.
func LinuxMedium(num int) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_STANDARD_16)
}

// Large Linux GCE instances.
func LinuxLarge(num int) *gce.Instance {
	return linuxSwarmingBot(num, gce.MACHINE_TYPE_HIGHCPU_64)
}

// Linux GCE instances with AMD CPUs (skbug.com/10269).
func LinuxAmd(num int) *gce.Instance {
	vm := linuxSwarmingBot(num, gce.MACHINE_TYPE_N2D_STANDARD_16)
	vm.MinCpuPlatform = gce.CPU_PLATFORM_AMD
	return vm
}

// Linux GCE instances with Skylake CPUs.
func LinuxSkylake(num int) *gce.Instance {
	vm := LinuxMedium(num)
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

// Medium Windows GCE instances.
func WinMedium(ctx context.Context, num int) (*gce.Instance, error) {
	return winSwarmingBot(ctx, num, gce.MACHINE_TYPE_STANDARD_16)
}

// Large Windows GCE instances.
func WinLarge(ctx context.Context, num int) (*gce.Instance, error) {
	return winSwarmingBot(ctx, num, gce.MACHINE_TYPE_HIGHCPU_64)
}

// Windows GCE instances.
func winSwarmingBot(ctx context.Context, num int, machineType string) (*gce.Instance, error) {
	setupScript, err := getWindowsSetupScript(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	vm := swarming20180406(fmt.Sprintf("skia-e-gce-%03d", num), machineType, gce.SERVICE_ACCOUNT_CHROMIUM_SWARM, setupScript, WIN_SOURCE_IMAGE)
	vm.BootDisk.SizeGb = 300
	vm.BootDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.DataDisks = nil
	vm.Os = gce.OS_WINDOWS
	return vm, nil
}

// getWindowsSetupScript returns the contents of the setup script for Windows GCE instances.
func getWindowsSetupScript(ctx context.Context) (string, error) {
	chromeBotSkoloPassword, err := getChromeBotSkoloPassword(ctx)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	setupScript := strings.Replace(setupWinPS1, "CHROME_BOT_PASSWORD", chromeBotSkoloPassword, -1)
	return setupScript, nil
}

// getChromeBotSkoloPassword retrieves the chrome-bot Skolo password from GCP Secrets.
func getChromeBotSkoloPassword(ctx context.Context) (string, error) {
	// Read secret using GCP Secrets.
	secretsOutput := bytes.Buffer{}
	cmd := &exec.Command{
		Name:           "gcloud",
		Args:           []string{"--project=skia-infra-public", "secrets", "versions", "access", "latest", "--secret=ansible-secret-vars"},
		CombinedOutput: &secretsOutput,
	}
	if err := exec.Run(ctx, cmd); err != nil {
		return "", skerr.Wrap(err)
	}

	// Parse the secrets.yml file inside the secret.
	secretsYml := struct {
		Secrets struct {
			SkoloPassword string `yaml:"skolo_password"`
		} `yaml:"secrets"`
	}{}
	if err := yaml.Unmarshal(secretsOutput.Bytes(), &secretsYml); err != nil {
		return "", skerr.Wrap(err)
	}
	return secretsYml.Secrets.SkoloPassword, nil
}
