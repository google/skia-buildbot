package buildbot_ingest

/*
	Loads data from build masters and pushes it into InfluxDB.
*/
import (
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/golang/glog"
	influxdb "github.com/influxdb/influxdb/client"
	"skia.googlesource.com/buildbot.git/go/buildbot"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
)

const (
	SERIES_BUILDBOT_BYCOMMIT = "buildbot.builds.bycommit"
)

var (
	COLUMNS_BUILDBOT_BYCOMMIT = []string{
		"mastername",
		"buildername",
		"buildnumber",
		"branch",
		"revision",
	}
)

// submitBuild writes a series of points into InfluxDB for the given Build.
// the data point is duplicated for each commit which was first seen in this
// build (ie. the blamelist). This is to facilitate querying Influx using
// "where revision = 'abc123'".
func submitBuildByCommit(dbClient *influxdb.Client, build *buildbot.Build, revList []*gitinfo.ShortCommit) error {
	points := [][]interface{}{}
	for _, commit := range revList {
		point := []interface{}{
			interface{}(build.MasterName),
			interface{}(build.BuilderName),
			interface{}(build.Number),
			interface{}(build.Branch),
			interface{}(commit.Hash),
		}
		points = append(points, point)
	}
	series := influxdb.Series{
		Name:    SERIES_BUILDBOT_BYCOMMIT,
		Columns: COLUMNS_BUILDBOT_BYCOMMIT,
		Points:  points,
	}
	return dbClient.WriteSeries([]*influxdb.Series{&series})
}

// retrySubmitBuildByCommit runs submitBuildByCommit in a loop until it
// succeeds or the maximum number of attempts is reached.
func retrySubmitBuildByCommit(dbClient *influxdb.Client, build *buildbot.Build, revList []*gitinfo.ShortCommit) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = submitBuildByCommit(dbClient, build, revList)
		if err == nil {
			return nil
		}
	}
	return err
}

// processBuildByCommit does the necessary work before submitting the build
// into InfluxDB.  This includes loading the build details from the master,
// determining which commits were first seen in the build, and then actually
// submitting the build into Influx.
func processBuildByCommit(dbClient *influxdb.Client, master, builder string, buildNum int, skiaRepo *gitinfo.GitInfo) error {
	glog.Infof("Processing %s -- %s #%d", master, builder, buildNum)
	b, err := retryGetBuild(master, builder, buildNum)
	if err != nil {
		return err
	}
	gotRevision := b.GotRevision
	if gotRevision == "" {
		return fmt.Errorf("Build has no got_revision: %v", b)
	}
	branch := b.Branch
	prevBuildNum, err := getLastProcessedBuildOnBranch(dbClient, builder, master, branch)
	if err != nil {
		return err
	}
	prevGotRevision := ""
	if prevBuildNum >= 0 {
		prevBuild, err := retryGetBuild(master, builder, prevBuildNum)
		if err != nil {
			return err
		}
		prevGotRevision = prevBuild.GotRevision
		if prevGotRevision == "" {
			return fmt.Errorf("Build has no got_revision: %v", b)
		}
	} else {
		prevGotRevision, err = skiaRepo.InitialCommit()
		if err != nil {
			return err
		}
		glog.Info("    using initial commit.")
	}
	revList, err := skiaRepo.ShortList(prevGotRevision, gotRevision)
	if err != nil {
		return fmt.Errorf("Unable to obtain commit list: %v", err)
	}
	glog.Infof("    %d revs: %v", len(revList.Commits), revList.Commits)
	if len(revList.Commits) < 1 {
		glog.Infof("    WARNING: No commits in range!?!?!?")
		glog.Infof("    %d: %s", prevBuildNum, prevGotRevision)
		glog.Infof("    %d: %s", buildNum, gotRevision)
		return nil
	}
	return retrySubmitBuildByCommit(dbClient, b, revList.Commits)
}

// getLastProcessedBuildOnBranch determines the number of the build on the
// given branch which was most recently added to InfluxDB. The commits on the
// branch between the returned build's GotRevision and the subsequent build's
// GotRevision are considered to be "first seen" in the subsequent build.
func getLastProcessedBuildOnBranch(dbClient *influxdb.Client, builder, master, branch string) (int, error) {
	q := fmt.Sprintf("select max(buildnumber) from %s where buildername='%s'", SERIES_BUILDBOT_BYCOMMIT, builder)
	if branch != "" {
		q = fmt.Sprintf("%s and branch='%s'", q, branch)
	}
	results, err := dbClient.Query(q)
	if err != nil {
		return -1, err
	}
	// Special case: if no results are returned, assume we've never seen this
	// builder and return -1.
	if len(results) == 0 {
		return -1, nil
	}

	// Error checking.
	if len(results) != 1 {
		return -1, fmt.Errorf("Query returned incorrect number of series: %q", q)
	}
	series := results[0]
	if series.Name != SERIES_BUILDBOT_BYCOMMIT {
		return -1, fmt.Errorf("Query returned the wrong series: %q; expected: %s got: %S", q, SERIES_BUILDBOT_BYCOMMIT, series.Name)
	}
	if len(series.Columns) != 2 {
		return -1, fmt.Errorf("Query returned incorrect number of columns: %q, %v", q, series.Columns)
	}
	if len(series.Points) != 1 {
		return -1, fmt.Errorf("Query returned more than one point: %q", q)
	}
	p := series.Points[0]
	if len(p) != len(series.Columns) {
		return -1, fmt.Errorf("Number of columns does not match number of fields in datapoint: %v, %v", p, series.Columns)
	}
	if len(p) != 2 {
		return -1, fmt.Errorf("Point contains incorrect number of fields: %v", p)
	}
	return int(p[1].(float64)), nil
}

// getLastProcessedBuild returns the number of the build most recently added
// to InfluxDB for the given builder.
func getLastProcessedBuild(dbClient *influxdb.Client, builder, master string) (int, error) {
	return getLastProcessedBuildOnBranch(dbClient, builder, master, "")
}

type buildRange struct {
	Start int
	End   int
}

func (b buildRange) Iter() <-chan int {
	ch := make(chan int)
	go func() {
		for i := b.Start; i < b.End; i++ {
			ch <- i
		}
		close(ch)
	}()
	return ch
}

// getBuildsToProcessByCommit returns a map indicating the range of unprocessed
// build numbers for each builder on each master. The keys are master names,
// values are sub-maps whose keys are builder names and values are two-element
// integer arrays indicating the first unprocessed and most recent build
// numbers for that builder. The maps do not include builders which have no
// unprocessed builds.
func getBuildsToProcessByCommit(dbClient *influxdb.Client) map[string]map[string]buildRange {
	buildsToProcess := map[string]map[string]buildRange{}
	var wg sync.WaitGroup
	for _, masterName := range buildbot.MasterNames {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			builders, err := buildbot.GetBuilders(m)
			if err != nil {
				glog.Error(err)
				return
			}
			for name, builder := range builders {
				// Get the latest build for this builder.
				latest := -1
				if len(builder.CachedBuilds) > 0 {
					latest = builder.CachedBuilds[len(builder.CachedBuilds)-1]
				}
				// Get last-processed build for this builder.
				lastProcessed, err := getLastProcessedBuild(dbClient, name, m)
				if err != nil {
					glog.Error(err)
					continue
				}
				if latest >= 0 && lastProcessed != latest {
					if _, ok := buildsToProcess[m]; !ok {
						buildsToProcess[m] = map[string]buildRange{}
					}
					buildsToProcess[m][name] = buildRange{
						Start: lastProcessed + 1,
						End:   latest + 1,
					}
				} else {
					glog.Infof("No new builds for %s. Last processed: %d, latest build: %d", name, lastProcessed, latest)
				}
			}
		}(masterName)
	}
	wg.Wait()
	return buildsToProcess
}

// LoadBuildbotByCommitData repeatedly loads data from the build masters and
// pushes it into InfluxDB. It is intended to be run as a goroutine.
func LoadBuildbotByCommitData(dbClient *influxdb.Client, workdir string) {
	skiaDir := path.Join(workdir, "skia")
	skiaRepo, err := gitinfo.CloneOrUpdate("https://skia.googlesource.com/skia.git", skiaDir, true)
	if err != nil {
		glog.Errorf("Failed to check out Skia: %s", err)
		return
	}
	for _ = range time.Tick(10 * time.Second) {
		buildsToProcess := getBuildsToProcessByCommit(dbClient)
		skiaRepo.Update(true, true)
		var wg sync.WaitGroup
		for m, buildsByBuilder := range buildsToProcess {
			for b, builds := range buildsByBuilder {
				glog.Infof("%s, %s", m, b)
				wg.Add(1)
				go func(master, builder string, r buildRange) {
					defer wg.Done()
					for i := range r.Iter() {
						err := processBuildByCommit(dbClient, master, builder, i, skiaRepo)
						if err != nil {
							glog.Error(err)
						}
					}
				}(m, b, builds)
			}
		}
		wg.Wait()
	}
}
