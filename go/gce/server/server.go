package server

/*
   Utilities for creating GCE instances to run servers.
*/

import (
	"context"
	"flag"
	"fmt"
	"sort"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/sklog"
)

var (
	// Flags.
	instance       = flag.String("instance", "", "Which instance to create/delete.")
	create         = flag.Bool("create", false, "Create the instance. Either --create or --delete is required.")
	delete         = flag.Bool("delete", false, "Delete the instance. Either --create or --delete is required.")
	deleteDataDisk = flag.Bool("delete-data-disk", false, "Delete the data disk. Only valid with --delete")
	ignoreExists   = flag.Bool("ignore-exists", false, "Do not fail out when creating a resource which already exists or deleting a resource which does not exist.")
	listContacts   = flag.Bool("list-contacts", false, "List contacts for each instance and exit.")
)

// Base config for server instances.
func Server20170928(name string) *gce.Instance {
	return &gce.Instance{
		BootDisk: &gce.Disk{
			Name:        name,
			SourceImage: "skia-pushable-base-v2017-09-28-000",
			Type:        gce.DISK_TYPE_PERSISTENT_STANDARD,
		},
		DataDisks: []*gce.Disk{{
			Name:      fmt.Sprintf("%s-data", name),
			MountPath: gce.DISK_MOUNT_PATH_DEFAULT,
			SizeGb:    300,
			Type:      gce.DISK_TYPE_PERSISTENT_STANDARD,
		}},
		MachineType:       gce.MACHINE_TYPE_HIGHMEM_16,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Os:                gce.OS_LINUX,
		Scopes: []string{
			auth.ScopeFullControl,
			auth.ScopeGerrit,
			auth.ScopePubsub,
			auth.ScopeUserinfoEmail,
			auth.ScopeUserinfoProfile,
		},
		ServiceAccount: gce.SERVICE_ACCOUNT_DEFAULT,
		Tags:           []string{"http-server", "https-server"},
		User:           gce.USER_DEFAULT,
	}
}

// Main takes a map of string -> gce.Instance to initialize in the given zone.
// The string keys are nicknames for the instances (e.g. "prod", "staging").
//  Only the instance specified by the --instance flag will be created.
func Main(zone string, instances map[string]*gce.Instance) {
	common.Init()

	if *listContacts {
		for instance, vm := range instances {
			fmt.Println(instance + ":")
			for _, contact := range vm.Contacts {
				fmt.Println("\t" + contact)
			}
		}
		return
	}

	vm, ok := instances[*instance]
	if !ok {
		validInstances := make([]string, 0, len(instances))
		for k := range instances {
			validInstances = append(validInstances, k)
		}
		sort.Strings(validInstances)
		sklog.Fatalf("Invalid --instance %q; name must be one of: %v", *instance, validInstances)
	}
	if *create == *delete {
		sklog.Fatal("Please specify --create or --delete, but not both.")
	}

	// Create the GCloud object.
	g, err := gce.NewLocalGCloud(gce.PROJECT_ID_SERVER, zone)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := g.CheckSsh(); err != nil {
		sklog.Fatal(err)
	}

	// Perform the requested operation.
	if *create {
		if err := g.CreateAndSetup(context.Background(), vm, *ignoreExists); err != nil {
			sklog.Fatal(err)
		}
	} else {
		if err := g.Delete(vm, *ignoreExists, *deleteDataDisk); err != nil {
			sklog.Fatal(err)
		}
	}
}
