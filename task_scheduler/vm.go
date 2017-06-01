package main

import (
	"path"
	"runtime"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/vm_server"
)

func TaskSchedulerBase(name, ipAddress string) *gce.Instance {
	vm := vm_server.Server20170518(name, ipAddress)
	vm.DataDisk.SizeGb = 1000
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.GSDownloads["gs://skia-buildbots/artifacts/server/.gitconfig"] = "/home/default/.gitconfig"
	vm.GSDownloads["gs://skia-buildbots/artifacts/server/.netrc"] = "/home/default/.netrc"
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "benjaminwagner"
	vm.Scopes = append(vm.Scopes,
		auth.SCOPE_GERRIT,
		auth.SCOPE_PUBSUB,
		auth.SCOPE_USERINFO_EMAIL,
		auth.SCOPE_USERINFO_PROFILE,
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

func TaskSchedulerTest() *gce.Instance {
	vm := TaskSchedulerBase("borenet-instance-creation-test", "104.154.112.141")
	return vm
}

func main() {
	vm_server.Main(map[string]*gce.Instance{
		"prod":     TaskSchedulerProd(),
		"internal": TaskSchedulerInternal(),
		"test":     TaskSchedulerTest(),
	})
}
