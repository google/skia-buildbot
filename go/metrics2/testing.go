package metrics2

import (
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/influxdb"
)

func InitForTesting() {
	if err := Init("testing", influxdb.NewTestClient()); err != nil {
		glog.Fatal(err)
	}
}
