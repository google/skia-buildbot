package main

import (
	"fmt"
	"path"
	"runtime"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func DebuggerBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].Name = fmt.Sprintf("%s-data", name)
	vm.DataDisks[0].SizeGb = 1000
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	vm.Metadata["owner_primary"] = "jcgregorio"

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.SetupScript = path.Join(dir, "setup-script.sh")
	return vm
}

func Prod() *gce.Instance {
	return DebuggerBase("skia-debugger")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod": Prod(),
	})
}
