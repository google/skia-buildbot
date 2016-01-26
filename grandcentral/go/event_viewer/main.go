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
	nsqdAddress  = flag.String("nsqd", "", "Address and port of nsqd instance.")
	gsBucket     = flag.String("gs_bucket", "skia-infra-gm", "bucket to listen to for storage events.")
	gsPrefix     = flag.String("gs_prefix", "dm-json-v1", "prefix to listen to for storage events.")
	botEventType = flag.String("bot_event_type", "", "bot event type to filter for.")
	botStepName  = flag.String("bot_step_name", "", "name of the step name we are interested in.")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> \n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Valid Commands:\n")
		fmt.Fprintf(os.Stderr, "   storage - Follow storage events.\n")
		fmt.Fprintf(os.Stderr, "   bot     - Follow build bot events.\n\n")
		flag.PrintDefaults()
	}
}

func main() {
	defer common.LogPanic()
	common.Init()

	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	args := flag.Args()
	glog.Infof("ARGS: %v", args)
	if len(args) != 1 {
		glog.Infof("Wrong number of arguments. Needed exactly 1.")
		flag.Usage()
		os.Exit(1)
	}

	if *nsqdAddress == "" {
		glog.Fatal("Missing address of nsqd server.")
	}

	globalEventBus, err := geventbus.NewNSQEventBus(*nsqdAddress)
	if err != nil {
		glog.Fatalf("Unable to connect to NSQ server: %s", err)
	}

	eventBus := eventbus.New(globalEventBus)

	switch args[0] {
	case "bot":
		filter := event.BotEventFilter().EventType(*botEventType).StepName(*botStepName)
		eventBus.SubscribeAsync(event.BuildbotEvents(filter), func(evData interface{}) {
			glog.Infof("Buildbot event:\n %v", evData)
		})
	case "storage":
		eventBus.SubscribeAsync(event.StorageEvent(*gsBucket, *gsPrefix), func(evData interface{}) {
			data := evData.(*event.GoogleStorageEventData)
			glog.Infof("Google Storage notification from bucket\n %s:  %s : %s", data.Updated, data.Bucket, data.Name)
		})
	default:
		flag.Usage()
		os.Exit(1)
	}

	select {}
}
