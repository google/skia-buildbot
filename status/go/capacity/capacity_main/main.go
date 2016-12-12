package main

import (
	"flag"
	"fmt"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/influxdb_init"
	"go.skia.org/infra/status/go/capacity"
)

var (
	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

func main() {
	common.Init()
	dbClient, err := influxdb_init.NewClientFromParamsAndMetadata(*influxHost, *influxUser, *influxPassword, *influxDatabase, true)

	if err != nil {
		fmt.Printf("Non nil error : %s\n", err)
		return
	}

	c := capacity.New(dbClient)
	err = c.QueryAll()
	if err != nil {
		fmt.Printf("Non nil error 2 : %s\n", err)
		return
	}
	//fmt.Println(c.CapacityMetrics())
}
