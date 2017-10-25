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
			Name:   fmt.Sprintf("%s-data", name),
			SizeGb: 100,
			Type:   gce.DISK_TYPE_PERSISTENT_STANDARD,
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
			MountPath: server.MOUNT_PATH_DEFAULT,
		},
		{
			Name:      fmt.Sprintf("%s-data-2", name),
			SizeGb:    7000,
			Type:      gce.DISK_TYPE_PERSISTENT_SSD,
			MountPath: "/mnt/pd0/data/imageStore/diffs",
		},
	}
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	return vm
}

func Prod() *gce.Instance {
	// Below IP has been whitelisted in skiaperf cloud DB.
	vm := GoldBase("skia-gold-prod", "35.194.17.199")
	vm.Metadata["auth_white_list"] = "google.com"
	return vm
}

func Pdfium() *gce.Instance {
	// Below IP has been whitelisted in skiaperf cloud DB.
	vm := GoldBase("skia-gold-pdfium", "104.197.62.179")
	vm.DataDisks[0].SizeGb = 500
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_16
	return vm
}

func Stage() *gce.Instance {
	vm := GoldBase("skia-gold-stage", "35.202.197.94")
	vm.Metadata["auth_white_list"] = "google.com"
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

func DiffServerStage() *gce.Instance {
	vm := DiffServerBase("skia-diffserver-stage")
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":             Prod(),
		"public":           Public(),
		"pdfium":           Pdfium(),
		"stage":            Stage(),
		"diffserver_prod":  DiffServerProd(),
		"diffserver_stage": DiffServerStage(),
	})
}
