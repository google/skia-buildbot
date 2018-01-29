package main

import (
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/server"
)

func IngestionTraceDBBase(name string) *gce.Instance {
	vm := server.Server20170928(name)
	vm.DataDisks[0].SizeGb = 1000
	vm.DataDisks[0].Type = gce.DISK_TYPE_PERSISTENT_STANDARD
	vm.MachineType = gce.MACHINE_TYPE_HIGHMEM_32
	vm.Metadata["owner_primary"] = "stephana"
	vm.Metadata["owner_secondary"] = "jcgregorio"
	return vm
}

func TraceDBProd() *gce.Instance {
	return IngestionTraceDBBase("skia-tracedb")
}

func IngestionProd() *gce.Instance {
	vm := IngestionTraceDBBase("skia-ingestion")
	vm.ServiceAccount = "gold-ingestion@skia-buildbots.google.com.iam.gserviceaccount.com"
	vm.DataDisks[0].SizeGb = 100
	vm.MachineType = gce.MACHINE_TYPE_STANDARD_16
	return vm
}

func main() {
	server.Main(gce.ZONE_DEFAULT, map[string]*gce.Instance{
		"tracedb":   TraceDBProd(),
		"ingestion": IngestionProd(),
	})
}
