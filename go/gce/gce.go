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
)

const (
	DISK_SNAPSHOT_SYSTEMD_PUSHABLE_BASE = "skia-systemd-pushable-base"

	DISK_TYPE_PERSISTENT_STANDARD = "pd-standard"
	DISK_TYPE_PERSISTENT_SSD      = "pd-ssd"

	MACHINE_TYPE_HIGHMEM_16 = "n1-highmem-16"

	MAINTENANCE_POLICY_MIGRATE = "MIGRATE"

	NETWORK_DEFAULT = "global/networks/default"

	OS_LINUX   = "Linux"
	OS_WINDOWS = "Windows"

	PROJECT_ID = "google.com:skia-buildbots"

	SERVICE_ACCOUNT_DEFAULT = "31977622648@project.gserviceaccount.com"

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

	// Optional, image or snapshot to flash to the disk.
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
	if disk.SourceSnapshot != "" {
		d.SourceSnapshot = fmt.Sprintf("projects/%s/global/snapshots/%s", g.project, disk.SourceSnapshot)
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

	// Files to download from Google Storage. Map keys are the source URLs,
	// and values are destination paths on the GCE instance. May be absolute
	// or relative (to the default user's home dir, eg. /home/default).
	GSDownloads map[string]string

	// GCE machine type specification, eg. "n1-standard-16".
	MachineType string

	// Instance-level metadata keys and values.
	Metadata map[string]string

	// Files to create based on metadata. Map keys are project-level
	// metadata keys, and values are destination paths on the GCE instance.
	// May be absolute or relative (to the default user's home dir, eg.
	// /home/default).
	MetadataDownloads map[string]string

	// Name of the instance.
	Name string

	// Auth scopes for the instance.
	Scopes []string

	// Path to a startup script for the instance, optional. Should be either
	// absolute or relative to the parent GCloud instance's workdir.
	StartupScript string

	// Tags for the instance.
	Tags []string

	// Default user name for the instance.
	User string
}

// CreateInstance creates the given VM instance.
func (g *GCloud) CreateInstance(vm *Instance, ignoreExists bool) error {
	sklog.Infof("Creating instance %q", vm.Name)
	if vm.Name == "" {
		return fmt.Errorf("Instance name is required.")
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
	metadata := make([]*compute.MetadataItems, 0, len(vm.Metadata))
	for k, v := range vm.Metadata {
		val := new(string)
		*val = v
		metadata = append(metadata, &compute.MetadataItems{
			Key:   k,
			Value: val,
		})
	}
	if vm.StartupScript != "" {
		var script string
		b, err := ioutil.ReadFile(vm.StartupScript)
		if err != nil {
			return err
		}
		script = string(b)
		metadata = append(metadata, &compute.MetadataItems{
			Key:   "startup-script",
			Value: &script,
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
			&compute.NetworkInterface{
				AccessConfigs: []*compute.AccessConfig{
					&compute.AccessConfig{
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
			&compute.ServiceAccount{
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
		if err := g.WaitForInstanceReady(vm, maxWaitTime); err != nil {
			return err
		}
		sklog.Infof("Successfully created instance %s", vm.Name)
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
			return fmt.Errorf("Exceeded timeout of %s", timeout)
		}
		sklog.Infof("Waiting for instance %q (status %s)", name, st)
		time.Sleep(5 * time.Second)
	}
	return nil
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
func (vm *Instance) Ssh(cmd ...string) (string, error) {
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
func (vm *Instance) Scp(src, dst string) error {
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
	if err := g.WaitForInstanceReady(vm, maxWaitTime); err != nil {
		return err
	}
	return nil
}

// WaitForInstanceReady waits until the instance is ready to use.
func (g *GCloud) WaitForInstanceReady(vm *Instance, timeout time.Duration) error {
	start := time.Now()
	if err := g.waitForInstance(vm.Name, instanceStatusRunning, timeout); err != nil {
		return err
	}
	for _, err := vm.Ssh("true"); err != nil; _, err = vm.Ssh("true") {
		if time.Now().Sub(start) > timeout {
			return fmt.Errorf("Exceeded timeout of %s", timeout)
		}
		sklog.Infof("Waiting for instance %q to be ready.", vm.Name)
		time.Sleep(5 * time.Second)
	}
	return nil
}

// DownloadFile downloads the given file from Google Cloud Storage to the
// instance.
func (vm *Instance) DownloadFile(src, dst string) error {
	_, err := vm.Ssh("gsutil", "cp", src, dst)
	return err
}

// GetFileFromMetadata downloads the given metadata entry to a file.
func (vm *Instance) GetFileFromMetadata(key, dst string) error {
	url := fmt.Sprintf(metadata.METADATA_URL, "project", key)
	_, err := vm.Ssh("wget", "--header", "'Metadata-Flavor: Google'", "--output-document", dst, url)
	return err
}

// SafeFormatAndMount copies the safe_format_and_mount script to the instance
// and runs it.
func (vm *Instance) SafeFormatAndMount() error {
	// Copy the format_and_mount.sh and safe_format_and_mount
	// scripts to the instance.
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	if err := vm.Scp(path.Join(dir, "format_and_mount.sh"), "/tmp/format_and_mount.sh"); err != nil {
		return err
	}
	if err := vm.Scp(path.Join(dir, "safe_format_and_mount"), "/tmp/safe_format_and_mount"); err != nil {
		return err
	}

	// Run format_and_mount.sh.
	if _, err := vm.Ssh("/tmp/format_and_mount.sh", vm.DataDisk.Name); err != nil {
		if !strings.Contains(err.Error(), "is already mounted") {
			return err
		}
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
		if err := g.CreateDisk(vm.DataDisk, ignoreExists); err != nil {
			return err
		}
	}
	if err := g.CreateInstance(vm, ignoreExists); err != nil {
		return err
	}

	// Fix ~/.gsutil permissions. They start out as root:root for some
	// reason, which prevents us from using gsutil at all.
	if _, err := vm.Ssh("sudo", "chown", "--recursive", fmt.Sprintf("%s:%s", vm.User, vm.User), ".gsutil"); err != nil {
		return err
	}

	// Format and mount.
	if vm.DataDisk != nil {
		if err := vm.SafeFormatAndMount(); err != nil {
			return err
		}
	}

	// GSutil downloads.
	for src, dst := range vm.GSDownloads {
		if err := vm.DownloadFile(src, dst); err != nil {
			return err
		}
	}

	// Metadata downloads.
	for key, dst := range vm.MetadataDownloads {
		if err := vm.GetFileFromMetadata(key, dst); err != nil {
			return err
		}
	}

	// Reboot the instance.
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
