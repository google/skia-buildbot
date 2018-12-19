package main

import (
	"path"
	"runtime"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func StatusBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].SizeGb = 100
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.Metadata["owner_primary"] = "borenet"
	vm.Metadata["owner_secondary"] = "kjlubick"
	vm.Scopes = append(vm.Scopes,
		bigtable.Scope,
		datastore.ScopeDatastore,
	)

	_, filename, _, _ := runtime.Caller(0)
	dir := path.Dir(filename)
	vm.StartupScript = path.Join(dir, "startup-script.sh")
	return vm
}

func StatusProd() *gce.Instance {
	vm := StatusBase("skia-status")
	vm.ServiceAccount = "status@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func StatusInternal() *gce.Instance {
	vm := StatusBase("skia-status-internal")
	vm.ServiceAccount = "status-internal@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func StatusStaging() *gce.Instance {
	vm := StatusBase("skia-status-staging")
	vm.ServiceAccount = "status@skia-buildbots.google.com.iam.gserviceaccount.com"
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"prod":     StatusProd(),
		"internal": StatusInternal(),
		"staging":  StatusStaging(),
	})
}
