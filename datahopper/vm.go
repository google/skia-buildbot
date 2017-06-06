package main

import (
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func DatahopperBase(name, ipAddress string) *gce.Instance {
	vm := server.AddGitConfigs(server.Server20170518(name), "skia-datahopper2")
	vm.DataDisk.SizeGb = 200
	vm.DataDisk.Type = gce.DISK_TYPE_PERSISTENT_SSD
	vm.ExternalIpAddress = ipAddress
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "jcgregorio"
	vm.Scopes = append(vm.Scopes,
		auth.SCOPE_USERINFO_EMAIL,
		auth.SCOPE_USERINFO_PROFILE,
	)
	return vm
}

func Prod() *gce.Instance {
	return DatahopperBase("skia-datahopper2", "104.154.112.122")
}

func Test1() *gce.Instance {
	return DatahopperBase("skia-datahopper-test1", "104.154.112.124")
}

func Test2() *gce.Instance {
	return DatahopperBase("skia-datahopper-test2", "104.154.112.125")
}

func main() {
	server.Main(map[string]*gce.Instance{
		"prod":  Prod(),
		"test1": Test1(),
		"test2": Test2(),
	})
}
