package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/geventbus"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/grandcentral/go/event"
)

// flags
var (
	nsqdAddress = flag.String("nsqd", "", "Address and port of nsqd instance.")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s bucket prefix \n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	defer common.LogPanic()
	common.Init()

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	bucket, prefix := args[0], args[1]
	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	if *nsqdAddress == "" {
		glog.Fatal("Missing address of nsqd server.")
	}

	globalEventBus, err := geventbus.NewNSQEventBus(*nsqdAddress)
	if err != nil {
		glog.Fatalf("Unable to connect to NSQ server: %s", err)
	}

	eventBus := eventbus.New(globalEventBus)
	eventBus.SubscribeAsync(event.StorageEvent(bucket, prefix), func(evData interface{}) {
		data := evData.(*event.GoogleStorageEventData)
		glog.Infof("Google Storage notification from bucket\n %s:  %s : %s", data.Updated, data.Bucket, data.Name)
	})
	select {}
}
