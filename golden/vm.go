package main

import (
	"fmt"
	"path"
	"runtime"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func GoldBase(name, ipAddress string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks = []*gce.Disk{
		&gce.Disk{
			Name:      fmt.Sprintf("%s-data", name),
			SizeGb:    100,
			Type:      gce.DISK_TYPE_PERSISTENT_STANDARD,
			MountPath: gce.DISK_MOUNT_PATH_DEFAULT,
		},
	}

	if ipAddress != "" {
		vm.ExternalIpAddress = ipAddress
	}
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	vm.Metadata["auth_white_list"] = "google.com chromium.org skia.org"
	vm.Metadata["owner_primary"] = "stephana"
	vm.Metadata["owner_secondary"] = "jcgregorio"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script.sh")
	return vm
}

// Define the base template for a diffserver instance.
func DiffServerBase(name string) *gce.Instance {
	// DiffServer uses an ephemeral IP address.
	vm := GoldBase(name, "")
	delete(vm.Metadata, "auth_white_list")
	vm.DataDisks = []*gce.Disk{
		{
			Name:      fmt.Sprintf("%s-data", name),
			SizeGb:    2000,
			Type:      gce.DISK_TYPE_PERSISTENT_SSD,
			MountPath: gce.DISK_MOUNT_PATH_DEFAULT,
		},
		{
			Name:      fmt.Sprintf("%s-data-2", name),
			SizeGb:    30000,
			Type:      gce.DISK_TYPE_PERSISTENT_SSD,
			MountPath: "/mnt/pd0/data/imageStore/diffs",
		},
	}
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	return vm
}

func Prod() *gce.Instance {
	// Below IP has been whitelisted in Cloud SQL.
	vm := GoldBase("skia-gold-prod", "35.194.17.199")
	vm.Metadata["auth_white_list"] = "google.com mtklein@chromium.org"
	return vm
}

func Pdfium() *gce.Instance {
	// Below IP has been whitelisted in Cloud SQL.
	vm := GoldBase("skia-gold-pdfium", "104.197.62.179")
	vm.DataDisks[0].SizeGb = 500
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_16
	return vm
}

func ChromeVR() *gce.Instance {
	// Below IP has been whitelisted in Cloud SQL.
	vm := GoldBase("skia-gold-chromevr", "35.224.220.244")
	vm.DataDisks[0].SizeGb = 500
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_16
	return vm
}

func Public() *gce.Instance {
	vm := GoldBase("skia-gold-public", "35.188.34.16")
	vm.Metadata["auth_white_list"] = `google.com
chromium.org
skia.org
kkinnunen@nvidia.com
mjk@nvidia.com
vbuzinov@nvidia.com
martina.kollarova@intel.com
this.is.harry.stern@gmail.com
dvonbeck@gmail.com
zakerinasab@chromium.org
afar.lin@imgtec.com`
	return vm
}

func DiffServerProd() *gce.Instance {
	return DiffServerBase("skia-diffserver-prod")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":            Prod(),
		"public":          Public(),
		"pdfium":          Pdfium(),
		"chromevr":        ChromeVR(),
		"diffserver_prod": DiffServerProd(),
	})
}
