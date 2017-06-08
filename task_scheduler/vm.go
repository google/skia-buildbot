package main

import (
	"path"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func TaskSchedulerBase(name, ipAddress string) *gce.Instance {
	vm := server.AddGitConfigs(server.Server20170518(name), name)
	vm.DataDisk.SizeGb = 1000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.ExternalIpAddress = ipAddress
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "benjaminwagner"
	vm.Scopes = append(vm.Scopes,
		auth.SCOPE_GERRIT,
	)

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.StartupScript = path.Join(dir, "startup-script.sh")
	return vm
}

func TaskSchedulerProd() *gce.Instance {
	return TaskSchedulerBase("skia-task-scheduler", "104.154.112.128")
}

func TaskSchedulerInternal() *gce.Instance {
	return TaskSchedulerBase("skia-task-scheduler-internal", "104.154.112.135")
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":     TaskSchedulerProd(),
		"internal": TaskSchedulerInternal(),
	})
}
