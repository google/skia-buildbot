package buildbot_ingest

/*
   Loads data from build masters and pushes it into InfluxDB.
*/

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	influxdb "github.com/influxdb/influxdb/client"
	"skia.googlesource.com/buildbot.git/go/buildbot"
)

const (
	SERIES_BUILDBOT_DETAILS = "buildbot.builds.details"
)

var (
	COLUMNS_BUILDBOT_DETAILS = []string{
		"mastername",
		"buildername",
		"buildnumber",
		"branch",
		"results",
		"revision",
		"started",
		"finished",
	}
)

// submitBuildDetails writes a single point into InfluxDB for the given Build.
func submitBuildDetails(dbClient *influxdb.Client, b *buildWrapper) error {
	var columns []string
	point := []interface{}{}
	if b.SequenceNumber != 0 && b.Time != 0 {
		columns = make([]string, len(COLUMNS_BUILDBOT_DETAILS)+2)
		columns[0] = "time"
		columns[1] = "sequence_number"
		for i, col := range COLUMNS_BUILDBOT_DETAILS {
			columns[i+2] = col
		}
		point = append(point, []interface{}{
			interface{}(b.Time),
			interface{}(b.SequenceNumber),
		}...)
	} else {
		columns = COLUMNS_BUILDBOT_DETAILS
	}

	point = append(point, []interface{}{
		interface{}(b.Build.MasterName),
		interface{}(b.Build.BuilderName),
		interface{}(b.Build.Number),
		interface{}(b.Build.Branch),
		interface{}(b.Build.Results),
		interface{}(b.Build.GotRevision),
		interface{}(b.Build.Times[0]),
		interface{}(b.Build.Times[1]),
	}...)
	series := influxdb.Series{
		Name:    SERIES_BUILDBOT_DETAILS,
		Columns: columns,
		Points:  [][]interface{}{point},
	}
	return dbClient.WriteSeriesWithTimePrecision([]*influxdb.Series{&series}, influxdb.Microsecond)
}

// retrySubmitBuildDetails runs submitBuildDetails in a loop until it succeeds
// or the maximum number of attempts is reached.
func retrySubmitBuildDetails(dbClient *influxdb.Client, build *buildWrapper) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = submitBuildDetails(dbClient, build)
		if err == nil {
			return nil
		}
	}
	return err
}

// retryGetBuild runs buildbot.GetBuild in a loop until it succeeds or the
// maximum number of attempts is reached.
func retryGetBuild(master, builder string, buildNum int) (*buildbot.Build, error) {
	var b *buildbot.Build
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		b, err = buildbot.GetBuild(master, builder, buildNum)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
		glog.Infof("Retrying %v #%v (attempt #%v)", builder, buildNum, attempt+2)
	}
	return b, err
}

// buildWrapper is a struct used to add InfluxDB timestamp and sequence_number
// fields to a buildbot.Build object.
type buildWrapper struct {
	Build          *buildbot.Build
	Time           int64
	SequenceNumber int64
}

// parseBuildsFromInflux takes a Series object from InfluxDB and returns
// a slice of buildWrapper objects.
func parseBuildsFromInflux(series *influxdb.Series) ([]*buildWrapper, error) {
	builds := []*buildWrapper{}
	for _, p := range series.Points {
		b := buildbot.Build{}
		b.Times = make([]float64, 2)
		var time int64
		var sequenceNumber int64
		for i, col := range series.Columns {
			if col == "time" {
				time = int64(p[i].(float64))
			} else if col == "sequence_number" {
				sequenceNumber = int64(p[i].(float64))
			} else if col == "mastername" {
				b.MasterName = p[i].(string)
			} else if col == "buildername" {
				b.BuilderName = p[i].(string)
			} else if col == "buildnumber" {
				b.Number = int(p[i].(float64))
			} else if col == "branch" {
				b.Branch = p[i].(string)
			} else if col == "results" {
				b.Results = int(p[i].(float64))
			} else if col == "revision" {
				b.GotRevision = p[i].(string)
			} else if col == "started" {
				b.Times[0] = p[i].(float64)
			} else if col == "finished" {
				b.Times[1] = p[i].(float64)
			}
		}
		builds = append(builds, &buildWrapper{&b, time, sequenceNumber})
	}
	return builds, nil
}

// getBuildFromInflux retrieves the given build from InfluxDB, if it exists.
// It returns a Build object, along with the timestamp and sequence_number of
// the build in the database.
func getBuildFromInflux(dbClient *influxdb.Client, master, builder string, buildNum int) (*buildWrapper, error) {
	q := fmt.Sprintf("select * from %s where mastername = '%s' and buildername = '%s' and buildnumber = %d", SERIES_BUILDBOT_DETAILS, master, builder, buildNum)
	results, err := dbClient.Query(q, influxdb.Microsecond)
	if err != nil {
		return nil, fmt.Errorf("Failed to load build from InfluxDB: %v", err)
	}
	// Special case: if no build is returned, assume it doesn't exist.
	if len(results) == 0 {
		return nil, nil
	}
	// Error checking.
	if len(results) != 1 {
		return nil, fmt.Errorf("Query returned incorrect number of series: %q %v", q, results)
	}
	series := results[0]
	if len(series.Points) != 1 {
		return nil, fmt.Errorf("Query returned incorrect number of builds: %q %v", q, series.Points)
	}
	builds, err := parseBuildsFromInflux(series)
	if err != nil {
		return nil, err
	}
	return builds[0], nil

}

// processBuildDetails does the necessary work befor submitting the build into
// InfluxDB.  This includes loading the build details from the master, loading
// previously-stored data about the build from InfluxDB, and then updating the
// build data in Influx.
func processBuildDetails(dbClient *influxdb.Client, master, builder string, buildNum int) error {
	glog.Infof("Processing %s -- %s #%d", master, builder, buildNum)
	b, err := retryGetBuild(master, builder, buildNum)
	if err != nil {
		return err
	}
	buildFromInflux, err := getBuildFromInflux(dbClient, master, builder, buildNum)
	if err != nil {
		return err
	}
	if buildFromInflux != nil {
		// Update the internals of the build with the newly-loaded data from the master.
		buildFromInflux.Build = b
		return retrySubmitBuildDetails(dbClient, buildFromInflux)
	} else {
		return retrySubmitBuildDetails(dbClient, &buildWrapper{
			Build: b,
		})
	}
}

// getUnfinishedBuilds loads all unfinished builds from InfluxDB and returns a
// slice of incomplete Build objects. The following fields are filled in:
// - BuilderName
// - Number
// - MasterName
func getUnfinishedBuilds(dbClient *influxdb.Client) ([]*buildbot.Build, error) {
	q := fmt.Sprintf("select * from %s where finished = 0", SERIES_BUILDBOT_DETAILS)
	results, err := dbClient.Query(q)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve unfinished builds: %v", err)
	}
	// Special case. If no results, assume there are no unfinished builds.
	if len(results) == 0 {
		return []*buildbot.Build{}, nil
	}
	// Error checking.
	if len(results) != 1 {
		return nil, fmt.Errorf("Query returned too many series: %q %v", q, results)
	}
	b := -1
	m := -1
	n := -1
	for i, col := range results[0].Columns {
		if col == "buildername" {
			b = i
		} else if col == "mastername" {
			m = i
		} else if col == "buildnumber" {
			n = i
		}
	}
	if b < 0 || m < 0 || n < 0 {
		return nil, fmt.Errorf("Not all columns are present in the query result: %v", results[0])
	}
	builds := []*buildbot.Build{}
	for _, p := range results[0].Points {
		if len(p) != len(results[0].Columns) {
			return nil, fmt.Errorf("Number of columns does not match number of fields in datapoint: %v, %v", p, results[0].Columns)
		}
		build := buildbot.Build{
			BuilderName: p[b].(string),
			MasterName:  p[m].(string),
			Number:      int(p[n].(float64)),
		}
		builds = append(builds, &build)
	}
	return builds, nil
}

// getBuildsToProcessDetails returns a map indicating the unprocessed build
// numbers for each builder on each master. The keys are master names, values
// are sub-maps whose keys are builder names and values are integer slices
// indicating the unprocessed build numbers for that builder. The maps do not
// include builders which have no unprocessed builds.
func getBuildsToProcessDetails(dbClient *influxdb.Client) map[string]map[string][]int {
	buildsToProcess := map[string]map[string][]int{}

	// Check for new builds.
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
						buildsToProcess[m] = map[string][]int{}
					}
					builds := make([]int, latest-lastProcessed)
					for i := lastProcessed + 1; i < latest+1; i++ {
						builds[i-lastProcessed-1] = i
					}
					buildsToProcess[m][name] = builds
				} else {
					glog.Infof("No new builds for %s", name)
				}
			}
		}(masterName)
	}
	wg.Wait()

	// Check for unfinished builds we've already seen.
	unfinished, err := getUnfinishedBuilds(dbClient)
	if err != nil {
		glog.Error(err)
	}
	glog.Infof("Processing %d unfinished builds.", len(unfinished))
	for _, b := range unfinished {
		if _, ok := buildsToProcess[b.MasterName]; !ok {
			buildsToProcess[b.MasterName] = map[string][]int{}
		}
		if _, ok := buildsToProcess[b.MasterName][b.BuilderName]; !ok {
			buildsToProcess[b.MasterName][b.BuilderName] = []int{}
		}
		buildsToProcess[b.MasterName][b.BuilderName] = append(buildsToProcess[b.MasterName][b.BuilderName], b.Number)
	}

	return buildsToProcess
}

// LoadBuildbotDetailsData repeatedly loads data from the build masters and pushes it
// into InfluxDB. It is intended to be run as a goroutine.
func LoadBuildbotDetailsData(dbClient *influxdb.Client) {
	for _ = range time.Tick(10 * time.Second) {
		buildsToProcess := getBuildsToProcessDetails(dbClient)
		var wg sync.WaitGroup
		for m, buildsByBuilder := range buildsToProcess {
			for b, builds := range buildsByBuilder {
				wg.Add(1)
				go func(master, builder string, builds []int) {
					defer wg.Done()
					for _, n := range builds {
						err := processBuildDetails(dbClient, master, builder, n)
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
