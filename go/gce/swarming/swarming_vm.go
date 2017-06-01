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

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GS_URL_GITCONFIG = "gs://skia-buildbots/artifacts/bots/.gitconfig"
	GS_URL_NETRC     = "gs://skia-buildbots/artifacts/bots/.netrc"

	IP_ADDRESS_TMPL = "104.154.112.%d"
	USER_CHROME_BOT = "chrome-bot"
)

var (
	// Flags.
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"2,3-10,22\"")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	internal       = flag.Bool("internal", false, "Whether or not the bots are internal.")
	windows        = flag.Bool("windows", false, "Whether or not the bots run Windows.")
	workdir        = flag.String("workdir", ".", "Working directory.")
)

// Base config for Swarming GCE instances.
func Swarming20170523(name, ipAddress string) *gce.Instance {
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
		GSDownloads:       map[string]string{},
		MachineType:       gce.MACHINE_TYPE_STANDARD_16,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		//MinCpuPlatform: ??? // TODO(borenet)
		Name: name,
		Os:   gce.OS_LINUX,
		Scopes: []string{
			auth.SCOPE_FULL_CONTROL,
		},
		Tags: []string{"http-server", "https-server"},
		User: USER_CHROME_BOT,
	}
}

// Linux GCE instances.
func LinuxSwarmingBot(name, ipAddress string) *gce.Instance {
	vm := Swarming20170523(name, ipAddress)
	vm.GSDownloads[GS_URL_GITCONFIG] = "/home/chrome-bot/.gitconfig"
	vm.GSDownloads[GS_URL_NETRC] = "/home/chrome-bot/.netrc"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script-linux.sh")

	return vm
}

// Windows GCE instances.
func WinSwarmingBot(name, ipAddress, pw, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	vm := Swarming20170523(name, ipAddress)
	vm.BootDisk.SizeGb = 300
	vm.BootDisk.SourceImage = "projects/google.com:windows-internal/global/images/windows-server-2008-r2-ent-internal-v20150310"
	vm.BootDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.DataDisk = nil
	// Most of the Windows setup, including the gitcookies, occurs in the
	// setup and startup scripts, which also install and schedule the
	// chrome-bot scheduled task script.
	vm.Metadata["chromebot-schtask-ps1"] = chromebotScript
	vm.Os = gce.OS_WINDOWS
	vm.Password = pw
	vm.SetupScript = setupScriptPath
	vm.StartupScript = startupScriptPath
	return vm
}

// Internal Swarming bots.
func InternalLinuxSwarmingBot(name, ipAddress string) *gce.Instance {
	vm := LinuxSwarmingBot(name, ipAddress)
	vm.MetadataDownloads[fmt.Sprintf(metadata.METADATA_URL, "project", "gitcookies_skia-internal_chromium")] = "/home/chrome-bot/.gitcookies"
	return vm
}

func InternalWinSwarmingBot(name, ipAddress, pw, setupScriptPath, startupScriptPath, chromebotScript string) *gce.Instance {
	return WinSwarmingBot(name, ipAddress, pw, setupScriptPath, startupScriptPath, chromebotScript)
}

// Returns the initial chrome-bot password, plus setup, startup, and
// chrome-bot scripts.
func getWindowsStuff(workdir string) (string, string, string, string, error) {
	pw, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/win-chrome-bot.txt")
	if err != nil {
		return "", "", "", "", err
	}
	pw = strings.TrimSpace(pw)

	_, filename, _, _ := runtime.Caller(0)
	repoRoot := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
	setupBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "win_setup.ps1"))
	if err != nil {
		return "", "", "", "", err
	}
	setupScript := strings.Replace(string(setupBytes), "CHROME_BOT_PASSWORD", pw, -1)
	setupPath := path.Join(workdir, "setup-script.ps1")
	if err := ioutil.WriteFile(setupPath, []byte(setupScript), os.ModePerm); err != nil {
		return "", "", "", "", err
	}

	netrcContents, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/.netrc")
	if err != nil {
		return "", "", "", "", err
	}
	setupScript = strings.Replace(setupScript, "INSERTFILE(/tmp/.netrc)", string(netrcContents), -1)

	gitconfigContents, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/.gitconfig")
	if err != nil {
		return "", "", "", "", err
	}
	setupScript = strings.Replace(setupScript, "INSERTFILE(/tmp/.gitconfig)", string(gitconfigContents), -1)

	startupBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "win_startup.ps1"))
	if err != nil {
		return "", "", "", "", err
	}
	startupScript := strings.Replace(string(startupBytes), "CHROME_BOT_PASSWORD", pw, -1)
	startupPath := path.Join(workdir, "startup-script.ps1")
	if err := ioutil.WriteFile(startupPath, []byte(startupScript), os.ModePerm); err != nil {
		return "", "", "", "", err
	}

	// Return the chrome-bot script itself, not its path.
	chromebotBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "chromebot-schtask.ps1"))
	if err != nil {
		return "", "", "", "", err
	}
	chromebotScript := util.ToDos(string(chromebotBytes))

	return pw, setupPath, startupPath, chromebotScript, nil
}

func main() {
	common.Init()
	defer common.LogPanic()

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
	g, err := gce.NewGCloud(gce.ZONE_DEFAULT, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}

	// Read the various Windows scripts.
	var pw, setupScript, startupScript, chromebotScript string
	if *windows {
		pw, setupScript, startupScript, chromebotScript, err = getWindowsStuff(wdAbs)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Perform the requested operation.
	group := util.NewNamedErrGroup()
	for _, num := range instanceNums {
		var vm *gce.Instance
		ipAddr := fmt.Sprintf(IP_ADDRESS_TMPL, num)
		if *windows {
			if *internal {
				name := fmt.Sprintf("skia-i-vm-%03d", num)
				vm = InternalWinSwarmingBot(name, ipAddr, pw, setupScript, startupScript, chromebotScript)
			} else {
				name := fmt.Sprintf("skia-vm-%03d", num)
				vm = WinSwarmingBot(name, ipAddr, pw, setupScript, startupScript, chromebotScript)
			}
		} else {
			if *internal {
				name := fmt.Sprintf("skia-i-vm-%03d", num)
				vm = InternalLinuxSwarmingBot(name, ipAddr)
			} else {
				name := fmt.Sprintf("skia-vm-%03d", num)
				vm = LinuxSwarmingBot(name, ipAddr)
			}
		}

		group.Go(vm.Name, func() error {
			if *create {
				if err := g.CreateAndSetup(vm, *ignoreExists, *workdir); err != nil {
					return err
				}

				if *windows {
					// Reboot. The startup script enabled auto-login as chrome-bot
					// on boot. Reboot in order to run chrome-bot's scheduled task.
					if err := g.Reboot(vm); err != nil {
						return err
					}
				} else {
					// Nothing to do.
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
