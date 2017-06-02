package gce

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"google.golang.org/api/compute/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	DISK_SNAPSHOT_SYSTEMD_PUSHABLE_BASE = "skia-systemd-pushable-base"

	DISK_TYPE_PERSISTENT_STANDARD = "pd-standard"
	DISK_TYPE_PERSISTENT_SSD      = "pd-ssd"

	MACHINE_TYPE_HIGHMEM_16  = "n1-highmem-16"
	MACHINE_TYPE_STANDARD_16 = "n1-standard-16"

	MAINTENANCE_POLICY_MIGRATE = "MIGRATE"

	NETWORK_DEFAULT = "global/networks/default"

	OS_LINUX   = "Linux"
	OS_WINDOWS = "Windows"

	PROJECT_ID = "google.com:skia-buildbots"

	SERVICE_ACCOUNT_DEFAULT = "31977622648@project.gserviceaccount.com"

	SETUP_SCRIPT_KEY_LINUX  = "setup-script"
	SETUP_SCRIPT_KEY_WIN    = "sysprep-oobe-script-ps1"
	SETUP_SCRIPT_PATH_LINUX = "/tmp/setup-script.sh"

	USER_DEFAULT = "default"

	ZONE_DEFAULT = "us-central1-c"

	diskStatusError = "ERROR"
	diskStatusReady = "READY"

	instanceStatusError   = "ERROR"
	instanceStatusRunning = "RUNNING"
	instanceStatusStopped = "TERMINATED"

	errNotFound      = "\\\"reason\\\": \\\"notFound\\\""
	errAlreadyExists = "\\\"reason\\\": \\\"alreadyExists\\\""

	maxWaitTime = 10 * time.Minute

	winSetupFinishedText   = "Instance setup finished."
	winStartupFinishedText = "Finished running startup scripts."
)

var (
	VALID_OS = []string{OS_LINUX, OS_WINDOWS}
)

// GCloud is a struct used for creating disks and instances in GCE.
type GCloud struct {
	project string
	s       *compute.Service
	workdir string
	zone    string
}

// NewGCloud returns a GCloud instance.
func NewGCloud(zone, workdir string) (*GCloud, error) {
	oauthCacheFile := path.Join(workdir, "gcloud_token.data")
	httpClient, err := auth.NewClient(true, oauthCacheFile, compute.CloudPlatformScope, compute.ComputeScope, compute.DevstorageFullControlScope)
	if err != nil {
		sklog.Fatal(err)
	}

	s, err := compute.New(httpClient)
	if err != nil {
		return nil, err
	}

	// Verify that we're set up for SSH.
	if _, err := sshArgs(); err != nil {
		return nil, err
	}

	return &GCloud{
		project: PROJECT_ID,
		s:       s,
		workdir: workdir,
		zone:    zone,
	}, nil
}

// Disk is a struct describing a disk resource in GCE.
type Disk struct {
	// The name of the disk.
	Name string

	// Size of the disk, in gigabytes.
	SizeGb int64

	// Optional, image to flash to the disk. Use only one of SourceImage
	// and SourceSnapshot.
	SourceImage string

	// Optional, snapshot to flash to the disk. Use only one of SourceImage
	// and SourceSnapshot.
	SourceSnapshot string

	// Type of disk, eg. "pd-standard" or "pd-ssd".
	Type string
}

// CreateDisk creates the given disk.
func (g *GCloud) CreateDisk(disk *Disk, ignoreExists bool) error {
	sklog.Infof("Creating disk %q", disk.Name)
	d := &compute.Disk{
		Name:   disk.Name,
		SizeGb: disk.SizeGb,
		Type:   fmt.Sprintf("zones/%s/diskTypes/%s", g.zone, disk.Type),
	}
	if disk.SourceImage != "" && disk.SourceSnapshot != "" {
		return fmt.Errorf("Only one of SourceImage and SourceSnapshot may be used.")
	}
	if disk.SourceImage != "" {
		if len(strings.Split(disk.SourceImage, "/")) == 5 {
			d.SourceImage = disk.SourceImage
		} else {
			d.SourceImage = fmt.Sprintf("projects/%s/global/images/%s", g.project, disk.SourceImage)
		}
	} else if disk.SourceSnapshot != "" {
		if len(strings.Split(disk.SourceSnapshot, "/")) == 5 {
			d.SourceSnapshot = disk.SourceSnapshot
		} else {
			d.SourceSnapshot = fmt.Sprintf("projects/%s/global/snapshots/%s", g.project, disk.SourceSnapshot)
		}
	}
	op, err := g.s.Disks.Insert(g.project, g.zone, d).Do()
	if err != nil {
		if strings.Contains(err.Error(), errAlreadyExists) {
			if ignoreExists {
				sklog.Infof("Disk %q already exists; ignoring.", disk.Name)
			} else {
				return fmt.Errorf("Disk %q already exists.", disk.Name)
			}
		} else {
			return err
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to insert disk: %v", op.Error)
	} else {
		if err := g.waitForDisk(disk.Name, diskStatusReady, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully created disk %s", disk.Name)
	}
	return nil
}

// DeleteDisk deletes the given disk.
func (g *GCloud) DeleteDisk(name string, ignoreNotExists bool) error {
	sklog.Infof("Deleting disk %q", name)
	op, err := g.s.Disks.Delete(g.project, g.zone, name).Do()
	if err != nil {
		if strings.Contains(err.Error(), errNotFound) {
			if ignoreNotExists {
				sklog.Infof("Disk %q does not exist; ignoring.", name)
			} else {
				return fmt.Errorf("Disk %q already exists.", name)
			}
		} else {
			sklog.Fatal(err)
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to delete disk: %v", op.Error)
	} else {
		if err := g.waitForDisk(name, diskStatusError, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully deleted disk %s", name)
	}
	return nil
}

// getDiskStatus returns the current status of the disk.
func (g *GCloud) getDiskStatus(name string) string {
	d, err := g.s.Disks.Get(g.project, g.zone, name).Do()
	if err != nil {
		return diskStatusError
	}
	return d.Status
}

// waitForDisk waits until the disk has the given status.
func (g *GCloud) waitForDisk(name, status string, timeout time.Duration) error {
	start := time.Now()
	for st := g.getDiskStatus(name); st != status; st = g.getDiskStatus(name) {
		if time.Now().Sub(start) > timeout {
			return fmt.Errorf("Exceeded timeout of %s", timeout)
		}
		sklog.Infof("Waiting for disk %q (status %s)", name, st)
		time.Sleep(5 * time.Second)
	}
	return nil
}

// Instance is a struct representing a GCE VM instance.
type Instance struct {
	// Information about the boot disk. Required.
	BootDisk *Disk

	// Information about an extra data disk. Optional.
	DataDisk *Disk

	// External IP address for the instance. Required.
	ExternalIpAddress string

	// Files to download from Google Storage. Map keys are destination paths
	// on the GCE instance and and values are the source URLs. Paths may be
	// absolute or relative (to the default user's home dir, eg.
	// /home/default).
	GSDownloads map[string]string

	// GCE machine type specification, eg. "n1-standard-16".
	MachineType string

	// Instance-level metadata keys and values.
	Metadata map[string]string

	// Files to create based on metadata. Map keys are destination paths on
	// the GCE instance and values are the source URLs (see
	// metadata.METADATA_URL). Paths May be absolute or relative (to the
	// default user's home dir, eg. /home/default).
	MetadataDownloads map[string]string

	// Name of the instance.
	Name string

	// Operating system of the instance.
	Os string

	// Password is the default user's password. Only used for Windows.
	Password string

	// Auth scopes for the instance.
	Scopes []string

	// Path to a setup script for the instance, optional. Should be either
	// absolute or relative to the parent GCloud instance's workdir. The
	// setup script runs once after the instance is created. For Windows,
	// this is assumed to be a PowerShell script and runs during sysprep.
	// For Linux, the script needs to be executable via the shell (ie. use
	// a shebang for Python scripts).
	SetupScript string

	// Path to a startup script for the instance, optional. Should be either
	// absolute or relative to the parent GCloud instance's workdir. The
	// startup script runs as root every time the instance starts up. For
	// Windows, this is assumed to be a PowerShell script. For Linux, the
	// script needs to be executable via the shell (ie. use a shebang for
	// Python scripts).
	StartupScript string

	// Tags for the instance.
	Tags []string

	// Default user name for the instance.
	User string
}

// scriptToMetadata reads the given script and inserts it into the Instance's
// metadata.
func scriptToMetadata(vm *Instance, key, path string) error {
	var script string
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	script = string(b)
	if vm.Os == OS_WINDOWS {
		script = util.ToDos(script)
	}
	vm.Metadata[key] = script
	return nil
}

// setupScriptToMetadata reads the setup script and returns a MetadataItems.
func setupScriptToMetadata(vm *Instance) error {
	key := SETUP_SCRIPT_KEY_WIN
	if vm.Os != OS_WINDOWS {
		key = SETUP_SCRIPT_KEY_LINUX
		vm.MetadataDownloads[SETUP_SCRIPT_PATH_LINUX] = fmt.Sprintf(metadata.METADATA_URL, "instance", SETUP_SCRIPT_KEY_LINUX)
	}
	return scriptToMetadata(vm, key, vm.SetupScript)
}

// startupScriptToMetadata reads the startup script and returns a MetadataItems.
func startupScriptToMetadata(vm *Instance) error {
	key := "startup-script"
	if vm.Os == OS_WINDOWS {
		key = "windows-startup-script-ps1"
	}
	return scriptToMetadata(vm, key, vm.StartupScript)
}

// createInstance creates the given VM instance.
func (g *GCloud) createInstance(vm *Instance, ignoreExists bool) error {
	sklog.Infof("Creating instance %q", vm.Name)
	if vm.Name == "" {
		return fmt.Errorf("Instance name is required.")
	}
	if vm.Os == "" {
		return fmt.Errorf("Instance OS is required.")
	}

	disks := []*compute.AttachedDisk{}
	if vm.BootDisk != nil {
		disks = append(disks, &compute.AttachedDisk{
			AutoDelete: true,
			Boot:       true,
			DeviceName: vm.BootDisk.Name,
			Source:     fmt.Sprintf("projects/%s/zones/%s/disks/%s", g.project, g.zone, vm.BootDisk.Name),
		})
	}
	if vm.DataDisk != nil {
		disks = append(disks, &compute.AttachedDisk{
			DeviceName: vm.DataDisk.Name,
			Source:     fmt.Sprintf("projects/%s/zones/%s/disks/%s", g.project, g.zone, vm.DataDisk.Name),
		})
	}
	if vm.Os == OS_WINDOWS && vm.User != "" && vm.Password != "" {
		vm.Metadata["gce-initial-windows-user"] = vm.User
		vm.Metadata["gce-initial-windows-password"] = vm.Password
	}
	if vm.SetupScript != "" {
		if err := setupScriptToMetadata(vm); err != nil {
			return err
		}
	}
	if vm.Os == OS_WINDOWS && vm.StartupScript != "" {
		// On Windows, the setup script runs automatically during
		// sysprep which is before the startup script runs. On Linux
		// the startup script does not run automatically, so to ensure
		// that the startup script runs after the setup script, we have
		// to wait to set the startup-script metadata item until after
		// we have manually run the setup script.
		if err := startupScriptToMetadata(vm); err != nil {
			return err
		}
	}
	metadata := make([]*compute.MetadataItems, 0, len(vm.Metadata))
	for k, v := range vm.Metadata {
		val := new(string)
		*val = v
		metadata = append(metadata, &compute.MetadataItems{
			Key:   k,
			Value: val,
		})
	}
	i := &compute.Instance{
		Disks:       disks,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", g.zone, vm.MachineType),
		Metadata: &compute.Metadata{
			Items: metadata,
		},
		Name: vm.Name,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				AccessConfigs: []*compute.AccessConfig{
					{
						NatIP: vm.ExternalIpAddress,
						Type:  "ONE_TO_ONE_NAT",
					},
				},
				Network: NETWORK_DEFAULT,
			},
		},
		Scheduling: &compute.Scheduling{
			OnHostMaintenance: MAINTENANCE_POLICY_MIGRATE,
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email:  SERVICE_ACCOUNT_DEFAULT,
				Scopes: vm.Scopes,
			},
		},
		Tags: &compute.Tags{
			Items: vm.Tags,
		},
	}
	op, err := g.s.Instances.Insert(g.project, g.zone, i).Do()
	if err != nil {
		if strings.Contains(err.Error(), errAlreadyExists) {
			if ignoreExists {
				sklog.Infof("Instance %q already exists; ignoring.", vm.Name)
			} else {
				return fmt.Errorf("Instance %q already exists.", vm.Name)
			}
		} else {
			return err
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to insert instance: %v", op.Error)
	} else {
		if err := g.waitForInstance(vm.Name, instanceStatusRunning, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully created instance %s", vm.Name)
	}
	// Obtain the instance IP address if necessary.
	if vm.ExternalIpAddress == "" {
		ip, err := g.GetIpAddress(vm)
		if err != nil {
			return err
		}
		vm.ExternalIpAddress = ip
	}
	if err := g.WaitForInstanceReady(vm, maxWaitTime); err != nil {
		return err
	}
	return nil
}

// DeleteInstance deletes the given GCE VM instance.
func (g *GCloud) DeleteInstance(name string, ignoreNotExists bool) error {
	sklog.Infof("Deleting instance %q", name)
	op, err := g.s.Instances.Delete(g.project, g.zone, name).Do()
	if err != nil {
		if strings.Contains(err.Error(), errNotFound) {
			if ignoreNotExists {
				sklog.Infof("Instance %q does not exist; ignoring.", name)
			} else {
				return fmt.Errorf("Instance %q does not exist.", name)
			}
		} else {
			sklog.Fatal(err)
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to delete instance: %v", op.Error)
	} else {
		if err := g.waitForInstance(name, instanceStatusError, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully deleted instance %s", name)
	}
	return nil
}

// getInstanceStatus returns the current status of the instance.
func (g *GCloud) getInstanceStatus(name string) string {
	i, err := g.s.Instances.Get(g.project, g.zone, name).Do()
	if err != nil {
		return instanceStatusError
	}
	return i.Status
}

// waitForInstance waits until the instance has the given status.
func (g *GCloud) waitForInstance(name, status string, timeout time.Duration) error {
	start := time.Now()
	for st := g.getInstanceStatus(name); st != status; st = g.getInstanceStatus(name) {
		if time.Now().Sub(start) > timeout {
			return fmt.Errorf("Instance did not have status %q within timeout of %s", status, timeout)
		}
		sklog.Infof("Waiting for instance %q (status %s)", name, st)
		time.Sleep(5 * time.Second)
	}
	return nil
}

// GetIpAddress obtains the IP address for the Instance.
func (g *GCloud) GetIpAddress(vm *Instance) (string, error) {
	inst, err := g.s.Instances.Get(g.project, g.zone, vm.Name).Do()
	if err != nil {
		return "", err
	}
	if len(inst.NetworkInterfaces) != 1 {
		return "", fmt.Errorf("Failed to obtain IP address: Instance has incorrect number of network interfaces: %d", len(inst.NetworkInterfaces))
	}
	if len(inst.NetworkInterfaces[0].AccessConfigs) != 1 {
		return "", fmt.Errorf("Failed to obtain IP address: Instance network interface has incorrect number of access configs: %d", len(inst.NetworkInterfaces[0].AccessConfigs))
	}
	ip := inst.NetworkInterfaces[0].AccessConfigs[0].NatIP
	if ip == "" {
		return "", fmt.Errorf("Failed to obtain IP address: Got empty IP address.")
	}
	return ip, nil
}

// sshArgs returns options for SSH or an error if applicable.
func sshArgs() ([]string, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}
	keyFile := path.Join(usr.HomeDir, ".ssh", "google_compute_engine")
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("You need to create an SSH key at %s, per https://cloud.google.com/compute/docs/instances/connecting-to-instance#generatesshkeypair", keyFile)
	}
	return []string{
		"-q", "-i", keyFile,
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "StrictHostKeyChecking=no",
	}, nil
}

// Ssh logs into the instance and runs the given command. Returns any output
// and an error if applicable.
func (g *GCloud) Ssh(vm *Instance, cmd ...string) (string, error) {
	if vm.Os == OS_WINDOWS {
		return "", fmt.Errorf("Cannot SSH into Windows machines (for: %v)", cmd)
	}
	if vm.ExternalIpAddress == "" {
		ip, err := g.GetIpAddress(vm)
		if err != nil {
			return "", err
		}
		vm.ExternalIpAddress = ip
	}
	args, err := sshArgs()
	if err != nil {
		return "", err
	}
	command := []string{"ssh"}
	command = append(command, args...)
	command = append(command, fmt.Sprintf("%s@%s", vm.User, vm.ExternalIpAddress))
	command = append(command, cmd...)
	sklog.Infof("Running %s", strings.Join(command, " "))
	return exec.RunCwd(".", command...)
}

// Scp copies files to the instance. The src argument is expected to be
// absolute.
func (g *GCloud) Scp(vm *Instance, src, dst string) error {
	if vm.Os == OS_WINDOWS {
		return fmt.Errorf("Cannot SCP to Windows machines (for: %s)", dst)
	}
	if vm.ExternalIpAddress == "" {
		ip, err := g.GetIpAddress(vm)
		if err != nil {
			return err
		}
		vm.ExternalIpAddress = ip
	}
	if !filepath.IsAbs(src) {
		return fmt.Errorf("%q is not an absolute path.", src)
	}
	args, err := sshArgs()
	if err != nil {
		return err
	}
	command := []string{"scp"}
	command = append(command, args...)
	command = append(command, src, fmt.Sprintf("%s@%s:%s", vm.User, vm.ExternalIpAddress, dst))
	sklog.Infof("Copying %s -> %s@%s:%s", src, vm.User, vm.Name, dst)
	_, err = exec.RunCwd(".", command...)
	return err
}

// Reboot stops and starts the instance.
func (g *GCloud) Reboot(vm *Instance) error {
	sklog.Infof("Rebooting instance %q", vm.Name)
	op, err := g.s.Instances.Stop(g.project, g.zone, vm.Name).Do()
	if err != nil {
		return err
	} else if op.Error != nil {
		return fmt.Errorf("Failed to stop instance: %v", op.Error)
	}
	if err := g.waitForInstance(vm.Name, instanceStatusStopped, maxWaitTime); err != nil {
		return err
	}
	op, err = g.s.Instances.Start(g.project, g.zone, vm.Name).Do()
	if err != nil {
		return err
	} else if op.Error != nil {
		return fmt.Errorf("Failed to start instance: %v", op.Error)
	}
	if err := g.waitForInstance(vm.Name, instanceStatusRunning, maxWaitTime); err != nil {
		return err
	}

	// Instance IP address may change at reboot.
	ip, err := g.GetIpAddress(vm)
	if err != nil {
		return err
	}
	vm.ExternalIpAddress = ip

	if err := g.WaitForInstanceReady(vm, maxWaitTime); err != nil {
		return err
	}
	return nil
}

// IsInstanceReady returns true iff the instance is ready.
func (g *GCloud) IsInstanceReady(vm *Instance) (bool, error) {
	if vm.Os == OS_WINDOWS {
		serial, err := g.s.Instances.GetSerialPortOutput(g.project, g.zone, vm.Name).Do()
		if err != nil {
			return false, err
		}
		if strings.Contains(serial.Contents, winStartupFinishedText) {
			return true, nil
		}
		if strings.Contains(serial.Contents, winSetupFinishedText) {
			return true, nil
		}
		return false, nil
	} else {
		if _, err := g.Ssh(vm, "true"); err != nil {
			return false, nil
		}
		return true, nil
	}
}

// WaitForInstanceReady waits until the instance is ready to use.
func (g *GCloud) WaitForInstanceReady(vm *Instance, timeout time.Duration) error {
	start := time.Now()
	if err := g.waitForInstance(vm.Name, instanceStatusRunning, timeout); err != nil {
		return err
	}
	for {
		if time.Now().Sub(start) > timeout {
			return fmt.Errorf("Instance was not ready within timeout of %s", timeout)
		}
		ready, err := g.IsInstanceReady(vm)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		sklog.Infof("Waiting for instance %q to be ready.", vm.Name)
		time.Sleep(5 * time.Second)
	}
}

// DownloadFile downloads the given file from Google Cloud Storage to the
// instance.
func (g *GCloud) DownloadFile(vm *Instance, src, dst string) error {
	_, err := g.Ssh(vm, "gsutil", "cp", src, dst)
	return err
}

// GetFileFromMetadata downloads the given metadata entry to a file.
func (g *GCloud) GetFileFromMetadata(vm *Instance, url, dst string) error {
	_, err := g.Ssh(vm, "wget", "--header", "'Metadata-Flavor: Google'", "--output-document", dst, url)
	return err
}

// SafeFormatAndMount copies the safe_format_and_mount script to the instance
// and runs it.
func (g *GCloud) SafeFormatAndMount(vm *Instance) error {
	// Copy the format_and_mount.sh and safe_format_and_mount
	// scripts to the instance.
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	if err := g.Scp(vm, path.Join(dir, "format_and_mount.sh"), "/tmp/format_and_mount.sh"); err != nil {
		return err
	}
	if err := g.Scp(vm, path.Join(dir, "safe_format_and_mount"), "/tmp/safe_format_and_mount"); err != nil {
		return err
	}

	// Run format_and_mount.sh.
	if _, err := g.Ssh(vm, "/tmp/format_and_mount.sh", vm.DataDisk.Name); err != nil {
		if !strings.Contains(err.Error(), "is already mounted") {
			return err
		}
	}
	return nil
}

// SetMetadata sets the given metadata on the instance.
func (g *GCloud) SetMetadata(vm *Instance, md map[string]string) error {
	items := make([]*compute.MetadataItems, 0, len(md))
	for k, v := range md {
		items = append(items, &compute.MetadataItems{
			Key:   k,
			Value: &v,
		})
	}
	op, err := g.s.Instances.SetMetadata(g.project, g.zone, vm.Name, &compute.Metadata{
		Items: items,
	}).Do()
	if err != nil {
		return err
	} else if op.Error != nil {
		return fmt.Errorf("Failed to set instance metadata: %v", op.Error)
	}
	return nil
}

// CreateAndSetup creates an instance and all its disks and performs any
// additional setup steps.
func (g *GCloud) CreateAndSetup(vm *Instance, ignoreExists bool, workdir string) error {
	// Create the disks and the instance.
	if vm.BootDisk != nil {
		if err := g.CreateDisk(vm.BootDisk, ignoreExists); err != nil {
			return err
		}
	}
	if vm.DataDisk != nil {
		if vm.Os == OS_WINDOWS {
			return fmt.Errorf("Data disks are not currently supported on Windows.")
		}
		if err := g.CreateDisk(vm.DataDisk, ignoreExists); err != nil {
			return err
		}
	}
	if err := g.createInstance(vm, ignoreExists); err != nil {
		return err
	}

	if vm.Os == OS_WINDOWS {
		// Set the metadata on the instance again, due to a bug
		// which is lost to time.
		if err := g.SetMetadata(vm, vm.Metadata); err != nil {
			return err
		}
	}

	// Format and mount.
	if vm.DataDisk != nil {
		if err := g.SafeFormatAndMount(vm); err != nil {
			return err
		}
	}

	// GSutil downloads.
	for dst, src := range vm.GSDownloads {
		if err := g.DownloadFile(vm, src, dst); err != nil {
			return err
		}
	}

	// Metadata downloads.
	for dst, src := range vm.MetadataDownloads {
		if err := g.GetFileFromMetadata(vm, src, dst); err != nil {
			return err
		}
	}

	// On Windows, the setup script runs automatically during sysprep. On
	// Linux, we have to run the setup script manually. In order to ensure
	// that the setup script runs before the startup script, we delay
	// setting the startup-script in metadata until after we've run the
	// setup script.
	if vm.Os != OS_WINDOWS {
		if vm.SetupScript != "" {
			if _, err := g.Ssh(vm, "sudo", "chmod", "+x", SETUP_SCRIPT_PATH_LINUX, "&&", SETUP_SCRIPT_PATH_LINUX); err != nil {
				return err
			}
		}
		if vm.StartupScript != "" {
			if err := startupScriptToMetadata(vm); err != nil {
				return err
			}
			if err := g.SetMetadata(vm, vm.Metadata); err != nil {
				return err
			}
		}
	}

	// Reboot the instance. On Windows, this will cause the startup script to run.
	if err := g.Reboot(vm); err != nil {
		return err
	}

	return nil
}

// Delete removes the instance and (maybe) its disks.
func (g *GCloud) Delete(vm *Instance, ignoreNotExists, deleteDataDisk bool) error {
	// Delete the instance. The boot disk will be auto-deleted.
	if err := g.DeleteInstance(vm.Name, true); err != nil {
		return err
	}
	// Only delete the data disk(s) if explicitly told to do so.
	if deleteDataDisk && vm.DataDisk != nil {
		if err := g.DeleteDisk(vm.DataDisk.Name, true); err != nil {
			return err
		}
	}
	return nil
}
