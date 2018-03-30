package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/skia-dev/go-systemd/dbus"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/systemd"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/push/go/types"
	"go.skia.org/infra/pushpull/go/command"
	"google.golang.org/api/option"
	storage "google.golang.org/api/storage/v1"
)

var (
	metadataTriggerCh = make(chan bool, 1)

	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// dbc is the dbus connection we use to talk to systemd.
	dbc *dbus.Conn

	hostname = ""

	ACTIONS = []string{
		string(command.Stop),
		string(command.Start),
		string(command.Restart),
	}

	// PROCESS_ENDING_UNITS include those that are likely to cause the
	// current process to end.
	PROCESS_ENDING_UNITS = []string{"reboot.target", "pulld.service"}
)

const (
	EXEC_MAIN_START_TS_PROP = "ExecMainStartTimestamp"
)

// flags
var (
	bucketName            = flag.String("bucket_name", "skia-push", "The name of the Google Storage bucket that contains push packages and info.")
	installedPackagesFile = flag.String("installed_packages_file", "installed_packages.json", "Path to the file where to cache the list of installed debs.")
	local                 = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port                  = flag.String("port", ":10000", "HTTP service address (e.g., ':8000')")
	promPort              = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir          = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	pullPeriod            = flag.Duration("pull_period", 5*time.Minute, "How often to check the configuration. On GCE, the metadata update will likely happen first")
)

type UnitStatusSlice []*systemd.UnitStatus

func (p UnitStatusSlice) Len() int           { return len(p) }
func (p UnitStatusSlice) Less(i, j int) bool { return p[i].Status.Name < p[j].Status.Name }
func (p UnitStatusSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func loadResouces() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}
}

func Init() {
	var err error
	dbc, err = dbus.New()
	if err != nil {
		sklog.Fatalf("Failed to initialize dbus: %s", err)
	}

	hostname, err = os.Hostname()
	if err != nil {
		sklog.Fatalf("Unable to retrieve hostname: %s", err)
	}
	packages.SetBucketName(*bucketName)
	loadResouces()
}

// getFunctionForAction returns StartUnit, StopUnit, or RestartUnit based on action.
func getFunctionForAction(action command.Action) func(name string, mode string, ch chan<- string) (int, error) {
	switch action {
	case command.Start:
		return dbc.StartUnit
	case command.Stop:
		return dbc.StopUnit
	case command.Restart:
		return dbc.RestartUnit
	default:
		sklog.Fatalf("%q in ACTIONS but not handled by getFunctionForAction", action)
		return nil
	}
}

// changeHandler changes the status of a service.
//
func changeHandler(cmd command.Command) string {
	f := getFunctionForAction(cmd.Action)
	if util.In(cmd.Service, PROCESS_ENDING_UNITS) {
		go func() {
			<-time.After(1 * time.Second)
			if _, err := f(cmd.Service, "replace", nil); err != nil {
				sklog.Error(err)
			}
		}()
		return "enqueued"
	} else {
		ch := make(chan string)
		if _, err := f(cmd.Service, "replace", ch); err != nil {
			sklog.Errorf("Error failed: %s", err)
			return "failed"
		}
		return <-ch
	}
}

// serviceOnly returns only units that are services.
func serviceOnly(units []*systemd.UnitStatus) []*systemd.UnitStatus {
	ret := []*systemd.UnitStatus{}
	for _, u := range units {
		if strings.HasSuffix(u.Status.Name, ".service") {
			ret = append(ret, u)
		}
	}
	return ret
}

// filterService returns only units with names in filter.
func filterService(units []*systemd.UnitStatus, filter map[string]bool) []*systemd.UnitStatus {
	ret := []*systemd.UnitStatus{}
	for _, u := range units {
		if filter[u.Status.Name] {
			ret = append(ret, u)
		}
	}
	return ret
}

// filterUnits fitlers down the units to only the interesting ones.
func filterUnits(units []*systemd.UnitStatus) []*systemd.UnitStatus {
	units = serviceOnly(units)
	sort.Sort(UnitStatusSlice(units))

	// Filter the list down to just services installed by push packages.
	installedPackages, err := packages.FromLocalFile(*installedPackagesFile)
	if err != nil {
		return units
	}
	allServices := map[string]bool{}
	for _, p := range installedPackages {
		for _, name := range allPackages[p].Services {
			allServices[name] = true
		}
	}
	return filterService(units, allServices)
}

func listUnits() ([]*systemd.UnitStatus, error) {
	unitStatus, err := dbc.ListUnits()
	units := make([]*systemd.UnitStatus, 0, len(unitStatus))
	if err == nil {
		for _, st := range unitStatus {
			cpst := st
			units = append(units, &systemd.UnitStatus{
				Status: &cpst,
			})
		}
	} else {
		return nil, fmt.Errorf("Failed to list units: %s", err)
	}
	if !*local {
		units = filterUnits(units)
	}
	// Now fill in the Props for each unit.
	for _, unit := range units {
		props, err := dbc.GetUnitTypeProperties(unit.Status.Name, "Service")
		if err != nil {
			sklog.Errorf("Failed to get props for the unit %s: %s", unit.Status.Name, err)
			continue
		}
		// Props are huge, only pass along the value(s) we use.
		unit.Props = map[string]interface{}{
			EXEC_MAIN_START_TS_PROP: props[EXEC_MAIN_START_TS_PROP],
		}
	}
	return units, nil
}

func pubSubList(ctx context.Context, ps *pubsub.Client) {
	// TODO make project into a flag.
	t := ps.Topic("status")
	for _ = range time.Tick(time.Minute) {

		units, err := listUnits()
		if err != nil {
			sklog.Errorf("Failed to list units: %s", err)
			continue
		}
		resp := &types.ListResponse{
			Hostname: hostname,
			Units:    units,
		}
		b, err := json.Marshal(resp)
		if err != nil {
			sklog.Errorf("Failed to encode output: %s", err)
			continue
		}
		pr := t.Publish(ctx, &pubsub.Message{
			Data: b,
		})
		<-pr.Ready()
	}
}

// Only returns on error.
func pubSubCommandWait(ctx context.Context, ps *pubsub.Client) {
	t := ps.Topic("command")
	sub := ps.Subscription("pulld")
	ok, err := sub.Exists(ctx)
	if err != nil {
		sklog.Fatal(err)
	}
	if !ok {
		sub, err = ps.CreateSubscription(ctx, "pulld", pubsub.SubscriptionConfig{
			Topic:       t,
			AckDeadline: 10 * time.Second,
		})
		if err != nil {
			sklog.Fatal(err)
		}
	}
	err = sub.Receive(ctx, func(innerCtx context.Context, m *pubsub.Message) {
		m.Ack()
		var cmd command.Command
		if err := json.Unmarshal(m.Data, &cmd); err != nil {
			sklog.Errorf("Failed to decode command: %s", err)
			return
		}
		if cmd.Action == command.Pull {
			metadataTriggerCh <- true
			sklog.Infof("Pull triggered via pub/sub.")
		} else if util.In(string(cmd.Action), ACTIONS) {
			// Store the response from below and record
			// cmd and timestamp for status.
			sklog.Infof("Change action triggered via pub/sub: %v", cmd)
			_ = changeHandler(cmd)
		} else {
			sklog.Errorf("Unknown pull action: %s", cmd.Action)
		}
	})
	if err != nil {
		sklog.Fatal(err)
	}
}

func main() {
	defer common.LogPanic()
	flag.Parse()
	common.InitWithMust(
		"pulld",
		common.PrometheusOpt(promPort),
		common.CloudLoggingDefaultAuthOpt(local),
	)
	tokenSource, err := auth.NewDefaultTokenSource(*local, storage.DevstorageFullControlScope, "https://www.googleapis.com/auth/pubsub")
	if err != nil {
		sklog.Fatal(err)
	}
	client := auth.ClientFromTokenSource(tokenSource)

	Init()

	ctx := context.Background()
	pullInit(ctx, client, metadataTriggerCh)
	rebootMonitoringInit()

	ps, err := pubsub.NewClient(ctx, "google.com:skia-push", option.WithTokenSource(tokenSource))
	if err != nil {
		sklog.Fatal(err)
	}
	go pubSubList(ctx, ps)
	pubSubCommandWait(ctx, ps)
}
