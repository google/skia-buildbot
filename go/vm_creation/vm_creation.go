package vm_creation

import (
	"fmt"
	"os"
	"os/user"
	"path"
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
	BOOT_DISK_SIZE_GB_LINUX = 20
	BOOT_DISK_SIZE_GB_WIN   = 300

	DISK_MODE_READ_ONLY  = "READ_ONLY"
	DISK_MODE_READ_WRITE = "READ_WRITE"

	DISK_SNAPSHOT_SYSTEMD_PUSHABLE_BASE = "skia-systemd-pushable-base"

	DISK_TYPE_PERSISTENT_STANDARD = "pd-standard"
	DISK_TYPE_PERSISTENT_SSD      = "pd-ssd"

	IMAGE_NAME_SWARMING_LINUX = "skia-swarming-v3"
	IMAGE_NAME_SWARMING_WIN   = "projects/google.com:windows-internal/global/images/windows-server-2008-r2-ent-internal-v20150310"
	IMAGE_NAME_CT             = "skia-buildbot-v8" // TODO(borenet): Is this needed/correct?

	MACHINE_TYPE_HIGHMEM_16 = "n1-highmem-16"

	MAINTENANCE_POLICY_MIGRATE = "MIGRATE"

	NETWORK_DEFAULT = "global/networks/default"

	OS_LINUX   = "Linux"
	OS_WINDOWS = "Windows"

	PERSISTENT_DISK_SIZE_GB    = int64(300)
	PERSISTENT_DISK_SIZE_GB_CT = int64(3000)

	PROJECT_ID = "google.com:skia-buildbots"

	SERVICE_ACCOUNT_DEFAULT = "31977622648@project.gserviceaccount.com"

	USER_CHROME_BOT = "chrome-bot"
	USER_DEFAULT    = "default"

	ZONE_DEFAULT = "us-central1-c"

	diskStatusError = "ERROR"
	diskStatusReady = "READY"

	instanceStatusError   = "ERROR"
	instanceStatusRunning = "RUNNING"
	instanceStatusStopped = "TERMINATED"

	errNotFound      = "\\\"reason\\\": \\\"notFound\\\""
	errAlreadyExists = "\\\"reason\\\": \\\"alreadyExists\\\""
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
	return &GCloud{
		project: PROJECT_ID,
		s:       s,
		workdir: workdir,
		zone:    zone,
	}, nil
}

// Disk is a struct describing a disk resource in GCE.
type Disk struct {
	AutoDelete     bool
	Boot           bool
	Image          string
	Mode           string
	Name           string
	SizeGb         int64
	SourceSnapshot string
	Type           string
}

// InsertDisk inserts the given disk.
func (g *GCloud) InsertDisk(disk *Disk, ignoreExists bool) error {
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
				sklog.Warningf("Disk %q already exists; ignoring.", disk.Name)
			} else {
				return fmt.Errorf("Disk %q already exists.", disk.Name)
			}
		} else {
			return err
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to insert disk: %v", op.Error)
	} else {
		if err := g.waitForDisk(disk.Name, diskStatusReady, 5*time.Minute); err != nil {
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
				sklog.Warningf("Disk %q does not exist; ignoring.", name)
			} else {
				return fmt.Errorf("Disk %q already exists.", name)
			}
		} else {
			sklog.Fatal(err)
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to delete disk: %v", op.Error)
	} else {
		if err := g.waitForDisk(name, diskStatusError, 5*time.Minute); err != nil {
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
	Disks             []*Disk
	ExternalIpAddress string
	Image             string
	MachineType       string
	MaintenancePolicy string
	Metadata          map[string]string
	MinCpuPlatform    string
	Name              string
	Network           string
	Os                string
	Scopes            []string
	Tags              []string
	User              string
}

// InsertInstance inserts the given VM instance.
func (g *GCloud) InsertInstance(vm *Instance, ignoreExists bool) error {
	sklog.Infof("Creating instance %q", vm.Name)
	if vm.Name == "" {
		return fmt.Errorf("Instance name is required.")
	}
	if !util.In(vm.Os, VALID_OS) {
		return fmt.Errorf("Unknown os %q; must be one of: %v", vm.Os, VALID_OS)
	}

	disks := make([]*compute.AttachedDisk, 0, len(vm.Disks))
	for _, d := range vm.Disks {
		disks = append(disks, &compute.AttachedDisk{
			AutoDelete: d.AutoDelete,
			Boot:       d.Boot,
			DeviceName: d.Name,
			Source:     fmt.Sprintf("projects/%s/zones/%s/disks/%s", g.project, g.zone, d.Name),
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
				sklog.Warningf("Instance %q already exists; ignoring.", vm.Name)
			} else {
				return fmt.Errorf("Instance %q already exists.", vm.Name)
			}
		} else {
			return err
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to insert instance: %v", op.Error)
	} else {
		if err := g.WaitForInstanceReady(vm, 5*time.Minute); err != nil {
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
				sklog.Warningf("Instance %q does not exist; ignoring.", name)
			} else {
				return fmt.Errorf("Instance %q does not exist.", name)
			}
		} else {
			sklog.Fatal(err)
		}
	} else if op.Error != nil {
		return fmt.Errorf("Failed to delete instance: %v", op.Error)
	} else {
		if err := g.waitForInstance(name, instanceStatusError, 5*time.Minute); err != nil {
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
func (g *GCloud) Ssh(vm *Instance, cmd []string) (string, error) {
	args, err := sshArgs()
	if err != nil {
		return "", err
	}
	command := []string{"ssh"}
	command = append(command, args...)
	command = append(command, fmt.Sprintf("%s@%s", vm.User, vm.ExternalIpAddress))
	command = append(command, cmd...)
	sklog.Infof("Running %s", strings.Join(command, " "))
	return exec.RunCwd(g.workdir, command...)
}

// Scp copies files to the instance.
func (g *GCloud) Scp(vm *Instance, src, dst string) error {
	args, err := sshArgs()
	if err != nil {
		return err
	}
	command := []string{"scp"}
	command = append(command, args...)
	command = append(command, src, fmt.Sprintf("%s@%s:%s", vm.User, vm.ExternalIpAddress, dst))
	sklog.Infof("Copying %s -> %s@%s:%s", src, vm.User, vm.Name, dst)
	_, err = exec.RunCwd(g.workdir, command...)
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
	if err := g.waitForInstance(vm.Name, instanceStatusStopped, 5*time.Minute); err != nil {
		return err
	}
	op, err = g.s.Instances.Start(g.project, g.zone, vm.Name).Do()
	if err != nil {
		return err
	} else if op.Error != nil {
		return fmt.Errorf("Failed to start instance: %v", op.Error)
	}
	if err := g.WaitForInstanceReady(vm, 5*time.Minute); err != nil {
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
	for _, err := g.Ssh(vm, []string{"true"}); err != nil; _, err = g.Ssh(vm, []string{"true"}) {
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
func (g *GCloud) DownloadFile(vm *Instance, src, dst string) error {
	_, err := g.Ssh(vm, []string{"gsutil", "cp", src, dst})
	return err
}

// GetFileFromMetadata downloads the given metadata entry to a file.
func (g *GCloud) GetFileFromMetadata(vm *Instance, key, dst string) error {
	url := fmt.Sprintf(metadata.METADATA_URL, "project", key)
	_, err := g.Ssh(vm, []string{"wget", "--header", "Metadata-Flavor: Google", "--output-document", dst, url})
	return err
}
