package vm_server

/*
   Utilities for creating GCE instances to run servers.
*/

import (
	"flag"
	"fmt"
	"path/filepath"
	"sort"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vm_creation"
)

var (
	// Flags.
	instance       = flag.String("instance", "", "Which instance to create/delete.")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	workdir        = flag.String("workdir", ".", "Working directory.")
)

// Base config for server instances.
func Server20170518(name, ipAddress string) *vm_creation.Instance {
	return &vm_creation.Instance{
		BootDisk: &vm_creation.Disk{
			Name:           name,
			SourceSnapshot: vm_creation.DISK_SNAPSHOT_SYSTEMD_PUSHABLE_BASE,
			Type:           vm_creation.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisk: &vm_creation.Disk{
			Name:   fmt.Sprintf("%s-data", name),
			SizeGb: 300,
			Type:   vm_creation.DISK_TYPE_PERSISTENT_STANDARD,
		},
		ExternalIpAddress: ipAddress,
		FormatAndMount:    true,
		GSDownloads:       map[string]string{},
		MachineType:       vm_creation.MACHINE_TYPE_HIGHMEM_16,
		MaintenancePolicy: vm_creation.MAINTENANCE_POLICY_MIGRATE,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Network:           vm_creation.NETWORK_DEFAULT,
		Os:                vm_creation.OS_LINUX,
		Scopes: []string{
			auth.SCOPE_FULL_CONTROL,
		},
		Tags: []string{"http-server", "https-server"},
		User: vm_creation.USER_DEFAULT,
	}
}

func Main(instances map[string]*vm_creation.Instance) {
	common.Init()
	defer common.LogPanic()

	vm, ok := instances[*instance]
	if !ok {
		validInstances := make([]string, 0, len(instances))
		for k, _ := range instances {
			validInstances = append(validInstances, k)
		}
		sort.Strings(validInstances)
		sklog.Fatalf("Invalid instance name %q; must be one of: %v", *instance, validInstances)
	}
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
	}

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the GCloud object.
	g, err := vm_creation.NewGCloud(vm_creation.ZONE_DEFAULT, wdAbs)
	if err != nil {
		sklog.Fatal(err)
	}

	// Perform the requested operation.
	if *create {
		if err := g.CreateAndSetup(vm, *ignoreExists, *workdir); err != nil {
			sklog.Fatal(err)
		}
	} else {
		if err := g.Delete(vm, *ignoreExists, *deleteDataDisk); err != nil {
			sklog.Fatal(err)
		}
	}
}
