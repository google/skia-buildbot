package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/build_scheduler/go/build_queue"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/util"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "buildbot_scheduler"

	// MASTER_BUILDERS_URL is the JSON endpoint for the builders list on
	// build masters.
	MASTER_BUILDERS_URL = "http://build.chromium.org/p/%s/json/builders"
)

var (
	// "Constants"

	// BOT_WHITELIST indicates which bots we can schedule.
	BOT_WHITELIST = []string{
		"Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-BuildBucket",
	}

	// MASTERS determines which masters we poll for builders.
	MASTERS = []string{
		"client.skia",
		"client.skia.android",
		"client.skia.compile",
		"client.skia.fyi",
		//"client.skia.internal",
	}

	// Flags.
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	local          = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	scoreDecay24Hr = flag.Float64("scoreDecay24Hr", 0.9, "Build candidate scores are penalized using exponential time decay, starting at 1.0. This is the desired value after 24 hours. Setting it to 1.0 causes commits not to be prioritized according to commit time.")
	scoreThreshold = flag.Float64("scoreThreshold", build_queue.DEFAULT_SCORE_THRESHOLD, "Don't schedule builds with scores below this threshold.")
	timePeriod     = flag.String("timePeriod", "2d", "Time period to use.")
	workdir        = flag.String("workdir", "workdir", "Working directory to use.")
)

// jsonGet fetches the given URL and decodes JSON into the given destination object.
func jsonGet(url string, rv interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %v", url, err)
	}
	defer util.Close(resp.Body)
	if err := json.NewDecoder(resp.Body).Decode(rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %v", err)
	}
	return nil
}

// builder is a struct used for retrieving information about builders
// from the masters.
type builder struct {
	Name          string `json:"..."`
	Master        string `json:"..."`
	PendingBuilds int    `json:"pendingBuilds"`
	State         string `json:"state"`
}

// getFreeBuilders returns a slice of names of builders which are free.
func getFreeBuilders() ([]*builder, error) {
	builders := map[string][]*builder{}
	errs := map[string]error{}
	var wg sync.WaitGroup
	for _, m := range MASTERS {
		wg.Add(1)
		go func(master string) {
			defer wg.Done()
			url := fmt.Sprintf(MASTER_BUILDERS_URL, master)
			b := map[string]*builder{}
			if err := jsonGet(url, &b); err != nil {
				errs[master] = err
				return
			}
			builderList := make([]*builder, 0, len(b))
			for builderName, builder := range b {
				builder.Name = builderName
				builder.Master = master
				builderList = append(builderList, builder)
			}
			builders[master] = builderList
		}(m)
	}
	wg.Wait()
	if len(errs) > 0 {
		errString := "Failed to get free builders:"
		for _, err := range errs {
			errString += fmt.Sprintf("\n%v", err)
		}
		return nil, fmt.Errorf(errString)
	}
	rv := []*builder{}
	for _, buildersForMaster := range builders {
		for _, b := range buildersForMaster {
			if util.In(b.Name, BOT_WHITELIST) {
				if b.PendingBuilds == 0 && b.State == "idle" {
					rv = append(rv, b)
				}
			}
		}
	}
	return rv, nil
}

// scheduleBuilds finds builders with no pending builds, pops the
// highest-priority builds for each from the queue, and requests builds using
// buildbucket.
func scheduleBuilds(q *build_queue.BuildQueue, bb *buildbucket.Client) error {
	var wg sync.WaitGroup

	var free []*builder
	var freeErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		free, freeErr = getFreeBuilders()
	}()

	var updateErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		updateErr = q.Update()
	}()

	wg.Wait()
	if updateErr != nil {
		return updateErr
	}
	if freeErr != nil {
		return freeErr
	}

	errs := []error{}
	for _, b := range free {
		build, err := q.Pop(b.Name)
		if err == build_queue.ERR_EMPTY_QUEUE {
			continue
		}
		scheduled, err := bb.RequestBuild(b.Name, b.Master, build.Commit, build.Repo, build.Author)
		if err != nil {
			errs = append(errs, err)
		} else {
			glog.Infof("Scheduled: %s @ %s, score = %0.3f, id = %s", b.Name, build.Commit[0:7], build.Score, scheduled.Id)
		}
	}
	if len(errs) > 0 {
		errString := "Got errors when scheduling builds:"
		for _, err := range errs {
			errString += fmt.Sprintf("\n%v", err)
		}
		return fmt.Errorf(errString)
	}
	return nil
}

func main() {
	// Setup flags.
	dbConf := buildbot.DBConfigFromFlags()

	// Global init.
	common.InitWithMetrics(APP_NAME, graphiteServer)

	// Parse the time period.
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		glog.Fatal(err)
	}

	// Initialize the buildbot database.
	if !*local {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	// Initialize the BuildBucket client.
	c, err := auth.NewClient(*local, path.Join(*workdir, "oauth_token_cache"), buildbucket.DEFAULT_SCOPES...)
	if err != nil {
		glog.Fatal(err)
	}
	bb := buildbucket.NewClient(c)

	// Build the queue.
	q, err := build_queue.NewBuildQueue(period, *workdir, *scoreThreshold, *scoreDecay24Hr, BOT_WHITELIST)
	if err != nil {
		glog.Fatal(err)
	}

	// Start scheduling builds in a loop.
	liveness := metrics.NewLiveness(APP_NAME)
	if err := scheduleBuilds(q, bb); err != nil {
		glog.Errorf("Failed to schedule builds: %v", err)
	}
	for _ = range time.Tick(time.Minute) {
		liveness.Update()
		if err := scheduleBuilds(q, bb); err != nil {
			glog.Errorf("Failed to schedule builds: %v", err)
		}
	}
}
