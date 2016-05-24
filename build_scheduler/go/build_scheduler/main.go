package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/build_scheduler/go/build_queue"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "buildbot_scheduler"
)

var (
	// "Constants"

	// BOT_BLACKLIST indicates which bots we cannot schedule.
	BOT_BLACKLIST = []*regexp.Regexp{
		regexp.MustCompile("^Housekeeper-Periodic-AutoRoll$"),
		regexp.MustCompile("^Housekeeper-Nightly-RecreateSKPs_Canary$"),
		regexp.MustCompile("^Housekeeper-Weekly-RecreateSKPs$"),
		regexp.MustCompile("^Linux Tests$"),
		regexp.MustCompile("^Mac10\\.9 Tests$"),
		regexp.MustCompile("^Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Debug-CT_DM_1m_SKPs$"),
		regexp.MustCompile("^Win7 Tests \\(1\\)"),
		buildbot.TRYBOT_REGEXP,
	}

	// MASTERS determines which masters we poll for builders.
	MASTERS = []string{
		"client.skia",
		"client.skia.android",
		"client.skia.compile",
		"client.skia.fyi",
		//"client.skia.internal",
	}

	// REPOS are the repositories to query.
	REPOS = []string{
		common.REPO_SKIA,
		common.REPO_SKIA_INFRA,
	}

	// Flags.
	buildbotDbHost = flag.String("buildbot_db_host", "skia-datahopper2:8000", "Where the Skia buildbot database is hosted.")
	local          = flag.Bool("local", false, "Whether we're running on a dev machine vs in production.")
	scoreDecay24Hr = flag.Float64("scoreDecay24Hr", 0.9, "Build candidate scores are penalized using exponential time decay, starting at 1.0. This is the desired value after 24 hours. Setting it to 1.0 causes commits not to be prioritized according to commit time.")
	scoreThreshold = flag.Float64("scoreThreshold", build_queue.DEFAULT_SCORE_THRESHOLD, "Don't schedule builds with scores below this threshold.")
	timePeriod     = flag.String("timePeriod", "4d", "Time period to use.")
	workdir        = flag.String("workdir", "workdir", "Working directory to use.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
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

type buildslaveSlice []*buildbot.BuildSlave

func (s buildslaveSlice) Len() int           { return len(s) }
func (s buildslaveSlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s buildslaveSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// getFreeBuildslaves returns a slice of names of buildslaves which are free.
func getFreeBuildslaves() ([]*buildbot.BuildSlave, error) {
	errMsg := "Failed to get free buildslaves: %v"
	// Get the set of builders for each master.
	builders, err := buildbot.GetBuilders()
	if err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}

	// Get the set of buildslaves for each master.
	slaves, err := buildbot.GetBuildSlaves()
	if err != nil {
		return nil, fmt.Errorf(errMsg, err)
	}
	// Flatten the buildslaves list.
	buildslaves := map[string]*buildbot.BuildSlave{}
	for _, slavesMap := range slaves {
		for slavename, slaveobj := range slavesMap {
			buildslaves[slavename] = slaveobj
		}
	}

	// Empty the buildslaves' Builders lists and only include builders not
	// in the blacklist.
	for _, bs := range buildslaves {
		bs.Builders = map[string][]int{}
	}

	// Map the builders to buildslaves.
	for _, b := range builders {
		// Only include builders in the whitelist, and those only if
		// there are no already-pending builds.
		if !util.AnyMatch(BOT_BLACKLIST, b.Name) && b.PendingBuilds == 0 {
			for _, slave := range b.Slaves {
				buildslaves[slave].Builders[b.Name] = nil
			}
		}
	}

	// Return the builders which are connected and idle.
	rv := []*buildbot.BuildSlave{}
	for _, s := range buildslaves {
		if len(s.RunningBuilds) == 0 && s.Connected {
			rv = append(rv, s)
		}
	}
	return rv, nil
}

// scheduleBuilds finds builders with no pending builds, pops the
// highest-priority builds for each from the queue, and requests builds using
// buildbucket.
func scheduleBuilds(q *build_queue.BuildQueue, bb *buildbucket.Client) error {
	errMsg := "Failed to schedule builds: %v"

	// Get the list of idle buildslaves, update the BuildQueue.
	var wg sync.WaitGroup
	var free []*buildbot.BuildSlave
	var freeErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		free, freeErr = getFreeBuildslaves()
	}()

	var updateErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		updateErr = q.Update()
	}()

	wg.Wait()
	if updateErr != nil {
		return fmt.Errorf(errMsg, updateErr)
	}
	if freeErr != nil {
		return fmt.Errorf(errMsg, freeErr)
	}

	// Sort the list of idle buildslaves, for saner log viewing.
	sort.Sort(buildslaveSlice(free))

	// Schedule builds on free buildslaves.
	errs := []error{}
	glog.Infof("Free buildslaves:")
	for _, s := range free {
		glog.Infof("\t%s", s.Name)
		builders := make([]string, 0, len(s.Builders))
		for b, _ := range s.Builders {
			builders = append(builders, b)
		}
		build, err := q.Pop(builders)
		if err == build_queue.ERR_EMPTY_QUEUE {
			continue
		}
		if *local {
			glog.Infof("Would schedule: %s @ %s, score = %0.3f", build.Builder, build.Commit[0:7], build.Score)
		} else {
			scheduled, err := bb.RequestBuild(build.Builder, s.Master, build.Commit, build.Repo, build.Author)
			if err != nil {
				errs = append(errs, err)
			} else {
				glog.Infof("Scheduled: %s @ %s, score = %0.3f, id = %s", build.Builder, build.Commit[0:7], build.Score, scheduled.Id)
			}
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
	defer common.LogPanic()

	// Global init.
	common.InitWithMetrics2(APP_NAME, influxHost, influxUser, influxPassword, influxDatabase, local)

	// Parse the time period.
	period, err := human.ParseDuration(*timePeriod)
	if err != nil {
		glog.Fatal(err)
	}

	// Initialize the buildbot database.
	db, err := buildbot.NewRemoteDB(*buildbotDbHost)
	if err != nil {
		glog.Fatal(err)
	}

	// Initialize the BuildBucket client.
	c, err := auth.NewClient(*local, path.Join(*workdir, "oauth_token_cache"), buildbucket.DEFAULT_SCOPES...)
	if err != nil {
		glog.Fatal(err)
	}
	bb := buildbucket.NewClient(c)

	// Build the queue.
	repos := gitinfo.NewRepoMap(*workdir)
	for _, r := range REPOS {
		if _, err := repos.Repo(r); err != nil {
			glog.Fatal(err)
		}
	}
	q, err := build_queue.NewBuildQueue(period, repos, *scoreThreshold, *scoreDecay24Hr, BOT_BLACKLIST, db)
	if err != nil {
		glog.Fatal(err)
	}

	// Start scheduling builds in a loop.
	liveness := metrics2.NewLiveness("time-since-last-successful-scheduling", nil)
	if err := scheduleBuilds(q, bb); err != nil {
		glog.Errorf("Failed to schedule builds: %v", err)
	}
	for _ = range time.Tick(time.Minute) {
		if err := scheduleBuilds(q, bb); err != nil {
			glog.Errorf("Failed to schedule builds: %v", err)
		} else {
			liveness.Reset()
		}
	}
}
