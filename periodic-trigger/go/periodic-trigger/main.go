package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/periodic"
	"go.skia.org/infra/go/sklog"
)

const (
	// Template used for creating unique IDs for instances of triggers.
	TRIGGER_TS = "2006-01-02"
)

var (
	local   = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project = flag.String("project", "", "GCE project in which to publish the pub/sub message.")
	trigger = flag.String("trigger", "", "Name of periodic trigger.")
)

func main() {
	common.Init()
	defer common.Defer()
	ts, err := auth.NewDefaultTokenSource(*local, periodic.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	// TODO(borenet): This ID is not necessarily unique; if the cron job is
	// significantly delayed, we might end up sending the same message twice
	// with different dates. It also doesn't allow for periods smaller than
	// 24 hours.
	id := fmt.Sprintf("%s-%s", *trigger, time.Now().UTC().Format(TRIGGER_TS))
	if err := periodic.Trigger(context.Background(), *trigger, id, ts); err != nil {
		sklog.Fatal(err)
	}
}
