package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vm_creation"
)

const (
	// Names for the different Task Scheduler GCE instances.
	VM_INSTANCE_PROD     = "prod"
	VM_INSTANCE_INTERNAL = "internal"
)

var (
	// VM_INSTANCES lists the valid Task Scheduler GCE instance names.
	VM_INSTANCES = []string{VM_INSTANCE_PROD, VM_INSTANCE_INTERNAL}

	// VM_CONFIGS maps instance names to configuration information for those
	// instances.
	VM_CONFIGS = map[string]config{
		/*VM_INSTANCE_PROD: config{
			InstanceName: "skia-task-scheduler",
			IpAddress:    "104.154.112.128",
		},
		VM_INSTANCE_INTERNAL: config{
			InstanceName: "skia-task-scheduler-internal",
			IpAddress:    "104.154.112.135",
		},*/
		"test": config{
			InstanceName: "borenet-vm-creation-test",
			IpAddress:    "104.154.112.141",
		},
	}

	// Flags.
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	instance       = flag.String("instance", "prod", fmt.Sprintf("Which instance to build. One of: %v", VM_INSTANCES))
	workdir        = flag.String("workdir", ".", "Working directory.")
)

// config is a struct containing configuration information for Task Scheduler
// GCE instances.
type config struct {
	InstanceName string
	IpAddress    string
}

func main() {
	common.Init()
	defer common.LogPanic()

	if *create == *delete {
		sklog.Fatalf("Please specify --create or --delete, but not both.")
	}

	cfg, ok := VM_CONFIGS[*instance]
	if !ok {
		sklog.Fatalf("Invalid instance %q", *instance)
	}

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Set up VM and disk info.
	g, err := vm_creation.NewGCloud(vm_creation.ZONE_DEFAULT, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}

	// Boot disk.
	boot := &vm_creation.Disk{
		AutoDelete:     true,
		Boot:           true,
		Mode:           vm_creation.DISK_MODE_READ_WRITE,
		Name:           cfg.InstanceName,
		SourceSnapshot: vm_creation.DISK_SNAPSHOT_SYSTEMD_PUSHABLE_BASE,
		Type:           vm_creation.DISK_TYPE_PERSISTENT_STANDARD,
	}

	// Data disk.
	data := &vm_creation.Disk{
		Name:   fmt.Sprintf("%s-data", cfg.InstanceName),
		SizeGb: 1000,
		Type:   vm_creation.DISK_TYPE_PERSISTENT_SSD,
	}

	// The VM instance.
	startupScript, err := ioutil.ReadFile(path.Join(dir, "startup-script.sh"))
	if err != nil {
		sklog.Fatal(err)
	}

	vm := &vm_creation.Instance{
		Disks:             []*vm_creation.Disk{boot, data},
		ExternalIpAddress: cfg.IpAddress,
		MachineType:       vm_creation.MACHINE_TYPE_HIGHMEM_16,
		MaintenancePolicy: vm_creation.MAINTENANCE_POLICY_MIGRATE,
		Metadata: map[string]string{
			"owner_primary":   "borenet",
			"owner_secondary": "benjaminwagner",
			"startup-script":  string(startupScript),
		},
		Name:    cfg.InstanceName,
		Network: vm_creation.NETWORK_DEFAULT,
		Os:      vm_creation.OS_LINUX,
		Scopes: []string{
			auth.SCOPE_FULL_CONTROL,
			auth.SCOPE_GERRIT,
			auth.SCOPE_PUBSUB,
			auth.SCOPE_USERINFO_EMAIL,
			auth.SCOPE_USERINFO_PROFILE,
		},
		Tags: []string{"http-server", "https-server"},
		User: vm_creation.USER_DEFAULT,
	}

	// Perform the requested operation.
	if *create {
		// Create the disks and the instance.
		if err := g.InsertDisk(boot, *ignoreExists); err != nil {
			sklog.Fatal(err)
		}
		if err := g.InsertDisk(data, *ignoreExists); err != nil {
			sklog.Fatal(err)
		}
		if err := g.InsertInstance(vm, *ignoreExists); err != nil {
			sklog.Fatal(err)
		}

		// Fix ~/.gsutil permissions.
		if _, err := g.Ssh(vm, []string{"sudo", "chown", "--recursive", "default:default", ".gsutil"}); err != nil {
			sklog.Fatal(err)
		}

		// Copy the format_and_mount.sh and safe_format_and_mount
		// scripts to the instance.
		commonDir := path.Join(path.Dir(dir), "compute_engine_scripts", "common")
		if err := g.Scp(vm, path.Join(commonDir, "format_and_mount.sh"), "/tmp/format_and_mount.sh"); err != nil {
			sklog.Fatal(err)
		}
		if err := g.Scp(vm, path.Join(commonDir, "safe_format_and_mount"), "/tmp/safe_format_and_mount"); err != nil {
			sklog.Fatal(err)
		}

		// Run format_and_mount.sh.
		if _, err := g.Ssh(vm, []string{"/tmp/format_and_mount.sh", vm.Name}); err != nil {
			if !strings.Contains(err.Error(), "is already mounted") {
				sklog.Fatal(err)
			}
		}

		// Download required files to the instance.
		if err := g.DownloadFile(vm, "gs://skia-buildbots/artifacts/server/.gitconfig", "/home/default/.gitconfig"); err != nil {
			sklog.Fatal(err)
		}
		if err := g.DownloadFile(vm, "gs://skia-buildbots/artifacts/server/.netrc", "/home/default/.netrc"); err != nil {
			sklog.Fatal(err)
		}

		// Reboot the instance.
		if err := g.Reboot(vm); err != nil {
			sklog.Fatal(err)
		}
	} else {
		// Delete the instance. The boot disk will be auto-deleted.
		if err := g.DeleteInstance(vm.Name, *ignoreExists); err != nil {
			sklog.Fatal(err)
		}
		// Only delete the data disk if explicitly told to do so.
		if *deleteDataDisk {
			if err := g.DeleteDisk(data.Name, *ignoreExists); err != nil {
				sklog.Fatal(err)
			}
		}
	}
}
