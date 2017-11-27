package server

/*
   Utilities for creating GCE instances to run servers.
*/

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"sort"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GS_URL_GITCOOKIES_TMPL         = "gs://skia-buildbots/artifacts/server/.gitcookies_%s"
	GS_URL_GITCONFIG               = "gs://skia-buildbots/artifacts/server/.gitconfig"
	GS_URL_NETRC_READONLY          = "gs://skia-buildbots/artifacts/server/.netrc_git-fetch-readonly"
	GS_URL_NETRC_READONLY_INTERNAL = "gs://skia-buildbots/artifacts/server/.netrc_git-fetch-readonly-internal"
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
		// Include read-only git creds on all servers.
		GSDownloads: []*gce.GSDownload{
			&gce.GSDownload{
				Source: GS_URL_GITCONFIG,
				Dest:   "~/.gitconfig",
			},
			&gce.GSDownload{
				Source: GS_URL_NETRC_READONLY,
				Dest:   "~/.netrc",
				Mode:   "600",
			},
		},
		MachineType:       gce.MACHINE_TYPE_HIGHMEM_16,
		Metadata:          map[string]string{},
		MetadataDownloads: map[string]string{},
		Name:              name,
		Os:                gce.OS_LINUX,
		Scopes: []string{
			auth.SCOPE_FULL_CONTROL,
			auth.SCOPE_PUBSUB,
			auth.SCOPE_USERINFO_EMAIL,
			auth.SCOPE_USERINFO_PROFILE,
		},
		ServiceAccount: gce.SERVICE_ACCOUNT_DEFAULT,
		Tags:           []string{"http-server", "https-server"},
		User:           gce.USER_DEFAULT,
	}
}

// Set configuration for servers which need read-only access to internal Git
// repos.
func SetGitCredsReadOnlyInternal(vm *gce.Instance) *gce.Instance {
	newGSDownloads := make([]*gce.GSDownload, 0, len(vm.GSDownloads)+2)
	for _, gsd := range vm.GSDownloads {
		if !util.In(gsd.Dest, []string{"~/.gitcookies", "~/.netrc"}) {
			newGSDownloads = append(newGSDownloads, gsd)
		}
	}
	vm.GSDownloads = append(newGSDownloads, &gce.GSDownload{
		Source: GS_URL_NETRC_READONLY_INTERNAL,
		Dest:   "~/.netrc",
		Mode:   "600",
	})
	return vm
}

// Set configuration for servers who commit to git.
func SetGitCredsReadWrite(vm *gce.Instance, gitUser string) *gce.Instance {
	newGSDownloads := make([]*gce.GSDownload, 0, len(vm.GSDownloads)+2)
	for _, gsd := range vm.GSDownloads {
		if !util.In(gsd.Dest, []string{"~/.gitcookies", "~/.netrc"}) {
			newGSDownloads = append(newGSDownloads, gsd)
		}
	}
	vm.GSDownloads = append(newGSDownloads, &gce.GSDownload{
		Source: fmt.Sprintf(GS_URL_GITCOOKIES_TMPL, gitUser),
		Dest:   "~/.gitcookies",
	})
	return vm
}

// Main takes a map of string -> gce.Instance to initialize in the given zone.
// The string keys are nicknames for the instances (e.g. "prod", "staging").
//  Only the instance specified by the --instance flag will be created.
func Main(zone string, instances map[string]*gce.Instance) {
	common.Init()
	defer common.LogPanic()

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

	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the GCloud object.
	g, err := gce.NewGCloud(gce.PROJECT_ID_SERVER, zone, wdAbs)
	if err != nil {
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
