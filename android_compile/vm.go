package main

import (
	"path"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

// Below dimensions were determined from
// https://source.android.com/setup/requirements
func Prod() *gce.Instance {
	vm := server.Server20170928("android-compile")
	// Recommended disk space (as of 1/10/18) was 250GB per checkout. Since the
	// server will hold 10 checkouts it will need min of 2.5TB. Using 5TB to be
	// safe.
	vm.DataDisks[0].SizeGb = 5000
	vm.DataDisks[0].MountPath = "/mnt/pd0"
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.ExternalIpAddress = "104.154.185.143" /* Whitelisted to checkout Android */
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	vm.Metadata["owner_primary"] = "rmistry"
	vm.Metadata["owner_secondary"] = "borenet"
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script.sh")
	vm.Scopes = append(vm.Scopes,
		auth.SCOPE_GERRIT,
	)
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
