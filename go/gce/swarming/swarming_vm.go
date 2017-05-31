package main

/*
   Program for automating creation and setup of Swarming bot VMs.
*/

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

const (
	IP_ADDRESS_TMPL = "104.154.112.%d"
	USER_CHROME_BOT = "chrome-bot"
)

var (
	// Flags.
	instances      = flag.String("instances", "", "Which instances to create/delete, eg. \"3-10\" or \"4,6,22\"")
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
	vm.GSDownloads["gs://skia-buildbots/artifacts/bots/.gitconfig"] = "/home/chrome-bot/.gitconfig"
	vm.GSDownloads["gs://skia-buildbots/artifacts/bots/.netrc"] = "/home/chrome-bot/.netrc"
	return vm
}

// Windows GCE instances.
func WinSwarmingBot(name, ipAddress, pw, sysprepScript, startupScript, chromebotScript string) *gce.Instance {
	vm := Swarming20170523(name, ipAddress)
	vm.BootDisk.SizeGb = 300
	vm.BootDisk.SourceImage = "projects/google.com:windows-internal/global/images/windows-server-2008-r2-ent-internal-v20150310"
	vm.BootDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.DataDisk = nil
	// Most of the Windows setup, including the gitcookies, occurs in the
	// sysprep and startup scripts.
	vm.Metadata["gce-initial-windows-user"] = USER_CHROME_BOT
	vm.Metadata["gce-initial-windows-password"] = pw
	vm.Metadata["sysprep-oobe-script-ps1"] = sysprepScript
	vm.Metadata["windows-startup-script-ps1"] = startupScript
	vm.Metadata["chromebot-schtask-ps1"] = chromebotScript
	vm.Os = gce.OS_WINDOWS
	return vm
}

// Internal Swarming bots.
func InternalLinuxSwarmingBot(name, ipAddress string) *gce.Instance {
	vm := Swarming20170523(name, ipAddress)
	vm.MetadataDownloads["gitcookies_skia-internal_chromium"] = "/home/chrome-bot/.gitcookies"
	return vm
}

func InternalWinSwarmingBot(name, ipAddress, pw, sysprepScript, startupScript, chromebotScript string) *gce.Instance {
	return WinSwarmingBot(name, ipAddress, pw, sysprepScript, startupScript, chromebotScript)
}

// Returns the initial chrome-bot password, plus sysprep, startup, and
// chrome-bot scripts.
func getWindowsStuff() (string, string, string, string, error) {
	pw, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/win-chrome-bot.txt")
	if err != nil {
		return "", "", "", "", err
	}

	_, filename, _, _ := runtime.Caller(0)
	repoRoot := path.Dir(path.Dir(path.Dir(path.Dir(filename))))
	sysprepBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "win_setup.ps1"))
	if err != nil {
		return "", "", "", "", err
	}
	sysprepScript := strings.Replace(string(sysprepBytes), "\n", "\r\n", -1)
	sysprepScript = strings.Replace(sysprepScript, "CHROME_BOT_PASSWORD", pw, -1)

	netrcContents, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/.netrc")
	if err != nil {
		return "", "", "", "", err
	}
	sysprepScript = strings.Replace(sysprepScript, "INSERTFILE(/tmp/.netrc)", string(netrcContents), -1)

	gitconfigContents, err := exec.RunCwd(".", "gsutil", "cat", "gs://skia-buildbots/artifacts/bots/.gitconfig")
	if err != nil {
		return "", "", "", "", err
	}
	sysprepScript = strings.Replace(sysprepScript, "INSERTFILE(/tmp/.gitconfig)", string(gitconfigContents), -1)

	startupBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "win_startup.ps1"))
	if err != nil {
		return "", "", "", "", err
	}
	startupScript := strings.Replace(string(startupBytes), "\n", "\r\n", -1)
	startupScript = strings.Replace(startupScript, "CHROME_BOT_PASSWORD", pw, -1)

	chromebotBytes, err := ioutil.ReadFile(path.Join(repoRoot, "scripts", "chromebot-schtask.ps1"))
	if err != nil {
		return "", "", "", "", err
	}
	chromebotScript := strings.Replace(string(chromebotBytes), "\n", "\r\n", -1)
	return pw, sysprepScript, startupScript, chromebotScript, nil
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
	var pw, sysprepScript, startupScript, chromebotScript string
	if *windows {
		pw, sysprepScript, startupScript, chromebotScript, err = getWindowsStuff()
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Perform the requested operation.
	var group errgroup.Group
	for _, num := range instanceNums {
		num := num
		group.Go(func() error {
			var vm *gce.Instance
			ipAddr := fmt.Sprintf(IP_ADDRESS_TMPL, num)
			if *windows {
				if *internal {
					name := fmt.Sprintf("skia-i-vm-%03d", num)
					vm = InternalWinSwarmingBot(name, ipAddr, pw, sysprepScript, startupScript, chromebotScript)
				} else {
					name := fmt.Sprintf("skia-vm-%03d", num)
					vm = WinSwarmingBot(name, ipAddr, pw, sysprepScript, startupScript, chromebotScript)
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
			if *create {
				if err := g.CreateAndSetup(vm, *ignoreExists, *workdir); err != nil {
					return err
				}

				if *windows {
					// Set the metadata on the instance again, due to a bug
					// which is lost to time.
					g.SetMetadata(vm, vm.Metadata)

					// Reboot, wait for startup script to run.
					if err := g.Reboot(vm); err != nil {
						return err
					}

					// 2. Reboot. The startup script enabled auto-login as chrome-bot
					//    on boot. Reboot in order to run chrome-bot's scheduled task.
					if err := g.Reboot(vm); err != nil {
						return err
					}
				} else {
					// 1. Install packages.
					pkgs := []string{
						"mercurial",
						"libosmesa-dev",
						"npm",
						"nodejs-legacy",
						"libexpat1-dev:i386",
						"clang-3.6",
						"poppler-utils",
						"netpbm",
					}
					cmd := append([]string{"sudo", "apt-get", "-y", "install"}, pkgs...)
					for _, pkg := range []string{"npm@3.10.9", "bower@1.6.5", "polylint@2.4.3"} {
						cmd = append(cmd, "&&", "sudo", "npm", "install", "-g", pkg)
					}
					cmd = append(cmd, []string{
						"&&", "wget", "https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb",
						"&&", "mkdir", "-p", "~/.config/google-chrome",
						"&&", "touch", "~/.config/google-chrome/First\\ Run",
						"&&", "(sudo", "dpkg", "-i", "google-chrome-stable_current_amd64.deb", "||", "sudo", "apt-get", "-f", "-y", "install)",
						"&&", "rm", "google-chrome-stable_current_amd64.deb",
					}...)
					cmd = append(cmd, []string{
						"&&", "sudo", "pip", "install", "coverage",
					}...)
					cmd = append(cmd, []string{
						"&&", "sudo", "apt-get", "-y", "--purge", "remove", "apache2*",
					}...)
					cmd = append(cmd, []string{
						"&&", "sudo", "sh", "-c", "\"echo '* - nofile 500000' >> /etc/security/limits.conf\"",
					}...)
					if _, err := vm.Ssh(cmd...); err != nil {
						return err
					}

					// 2. Fix depot tools.
					cmd = []string{
						"if", "[", "!", "-d", "depot_tools/.git", "];", "then",
						"rm", "-rf", "depot_tools;",
						"git", "clone", "https://chromium.googlesource.com/chromium/tools/depot_tools.git;",
						"fi",
					}
					if _, err := vm.Ssh(cmd...); err != nil {
						return err
					}

					// 3. Setup symlinks.
					cmd = []string{
						"sudo", "ln", "-s", "-f", "/usr/bin/clang-3.6", "/usr/bin/clang", "&&",
						"sudo", "ln", "-s", "-f", "/usr/bin/clang++-3.6", "/usr/bin/clang++", "&&",
						"sudo", "ln", "-s", "-f", "/usr/bin/llvm-cov-3.6", "/usr/bin/llvm-cov", "&&",
						"sudo", "ln", "-s", "-f", "/usr/bin/llvm-profdata-3.6", "/usr/bin/llvm-profdata",
					}
					if _, err := vm.Ssh(cmd...); err != nil {
						return err
					}

					// 4. Run Swarming bootstrap.
					swarm := swarming.SWARMING_SERVER
					if *internal {
						swarm = swarming.SWARMING_SERVER_PRIVATE
					}
					cmd = []string{
						"sudo", "chmod", "777", "/b", "&&",
						"mkdir", "-p", "/b/s", "&&",
						"wget", fmt.Sprintf("%s/bot_code", swarm), "-O", "/b/s/swarming_bot.zip", "&&",
						"ln", "-sf", "/b/s", "/b/swarm_slave",
					}
					if _, err := vm.Ssh(cmd...); err != nil {
						return err
					}

					// Reboot.
					if err := g.Reboot(vm); err != nil {
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
