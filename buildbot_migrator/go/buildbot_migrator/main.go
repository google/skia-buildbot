package main

import (
	"flag"
	"fmt"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/buildbot_deprecated"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
)

var (
	chunkSize      = flag.Int("chunk_size", 1000, "Number of builds to migrate at a time.")
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	newBuildbotDB  = flag.String("new_buildbot_db", "", "Path to the new buildbot DB.")
	testing        = flag.Bool("testing", false, "Set to true for local testing.")
)

type buildId struct {
	Id      int
	Master  string
	Builder string
	Number  int
}

// getAllBuildIDs returns a slice containing the ID for every build in the old
// buildbot database.
func getAllBuildIDs() ([]*buildId, error) {
	var ids []*buildId
	if err := buildbot_deprecated.DB.Select(&ids, fmt.Sprintf("SELECT id,master,builder,number from %s", buildbot_deprecated.TABLE_BUILDS)); err != nil {
		return nil, fmt.Errorf("Failed to obtain build IDs: %v", err)
	}
	return ids, nil
}

// convertBuild converts an old-style build to a new-style build.
func convertBuild(old *buildbot_deprecated.Build) *buildbot.Build {
	newSteps := make([]*buildbot.BuildStep, 0, len(old.Steps))
	for _, s := range old.Steps {
		newSteps = append(newSteps, &buildbot.BuildStep{
			Name:     s.Name,
			Number:   s.Number,
			Results:  s.Results,
			Started:  util.UnixFloatToTime(s.Started),
			Finished: util.UnixFloatToTime(s.Finished),
		})
	}

	newComments := make([]*buildbot.BuildComment, 0, len(old.Comments))
	for _, c := range old.Comments {
		newComments = append(newComments, &buildbot.BuildComment{
			Id:        int64(c.Id),
			User:      c.User,
			Timestamp: util.UnixFloatToTime(c.Timestamp),
			Message:   c.Message,
		})
	}

	return &buildbot.Build{
		Builder:       old.Builder,
		Master:        old.Master,
		Number:        old.Number,
		BuildSlave:    old.BuildSlave,
		Branch:        old.Branch,
		Commits:       old.Commits,
		GotRevision:   old.GotRevision,
		Properties:    old.Properties,
		PropertiesStr: old.PropertiesStr,
		Results:       old.Results,
		Steps:         newSteps,
		Started:       util.UnixFloatToTime(old.Started),
		Finished:      util.UnixFloatToTime(old.Finished),
		Comments:      newComments,
		Repository:    old.Repository,
	}
}

func main() {
	defer common.LogPanic()

	// Setup flags.
	dbConf := buildbot_deprecated.DBConfigFromFlags()

	common.InitWithMetrics("buildbot_migrator", graphiteServer)

	// Initialize the deprecated buildbot DB.
	if !*testing {
		if err := dbConf.GetPasswordFromMetadata(); err != nil {
			glog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		glog.Fatal(err)
	}

	// Initialize the new buildbot DB.
	newDB, err := buildbot.NewLocalDB(*newBuildbotDB)
	if err != nil {
		glog.Fatal(err)
	}

	// Determine which builds need to be copied.
	glog.Infof("Obtaining build IDs...")
	oldIds, err := getAllBuildIDs()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Got %d ids; checking already-migrated builds...", len(oldIds))
	ids := make([]int, 0, len(oldIds))
	for _, id := range oldIds {
		exists, err := newDB.BuildExists(id.Master, id.Builder, id.Number)
		if err != nil {
			glog.Fatal(err)
		}
		if !exists {
			ids = append(ids, id.Id)
		}
	}
	glog.Infof("Migrating %d builds...", len(ids))

	// Build insertion goroutine.
	builds := make(chan []*buildbot.Build)
	migrated := 0
	go func() {
		for newBuilds := range builds {
			if err := newDB.PutBuilds(newBuilds); err != nil {
				glog.Fatal(err)
			}
			migrated += len(newBuilds)
			glog.Infof("Migrated %d of %d", migrated, len(ids))
		}
	}()

	// Load builds from the old DB.
	if err := util.ChunkIter(ids, *chunkSize, func(chunk []int) error {
		oldBuilds, err := buildbot_deprecated.GetBuildsFromDB(chunk)
		if err != nil {
			return err
		}
		newBuilds := make([]*buildbot.Build, 0, len(oldBuilds))
		for _, b := range oldBuilds {
			newBuilds = append(newBuilds, convertBuild(b))
		}
		builds <- newBuilds

		return nil
	}); err != nil {
		glog.Fatal(err)
	}
}
