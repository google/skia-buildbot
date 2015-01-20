/*
	Pulls data from multiple sources and funnels into InfluxDB.
*/

package main

import (
	"flag"
	"fmt"
	"path"
	"regexp"
	"time"

	influxdb "github.com/influxdb/influxdb/client"
	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/datahopper/go/autoroll_ingest"
	"skia.googlesource.com/buildbot.git/go/buildbot"
	"skia.googlesource.com/buildbot.git/go/common"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
	"skia.googlesource.com/buildbot.git/go/metadata"
)

const (
	SKIA_REPO = "https://skia.googlesource.com/skia"
)

// flags
var (
	graphiteServer   = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	influxDbHost     = flag.String("influxdb_host", "localhost:8086", "The InfluxDB hostname.")
	influxDbName     = flag.String("influxdb_name", "root", "The InfluxDB username.")
	influxDbPassword = flag.String("influxdb_password", "root", "The InfluxDB password.")
	influxDbDatabase = flag.String("influxdb_database", "", "The InfluxDB database.")
	workdir          = flag.String("workdir", ".", "Working directory used by data processors.")
	local            = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	// Regexp matching non-alphanumeric characters.
	re = regexp.MustCompile("[^A-Za-z0-9]+")
)

// fixName transforms names of builders/buildsteps into strings useable by
// InfluxDB.
func fixName(s string) string {
	return re.ReplaceAllString(s, "_")
}

func main() {
	// Setup DB flags.
	database.SetupFlags(buildbot.PROD_DB_HOST, buildbot.PROD_DB_PORT, database.USER_RW, buildbot.PROD_DB_NAME)

	// Global init to initialize glog and parse arguments.
	common.InitWithMetrics("datahopper", graphiteServer)

	// Prepare the InfluxDB credentials. Load from metadata if appropriate.
	if !*local {
		*influxDbName = metadata.Must(metadata.ProjectGet(metadata.INFLUXDB_NAME))
		*influxDbPassword = metadata.Must(metadata.ProjectGet(metadata.INFLUXDB_PASSWORD))
	}
	dbClient, err := influxdb.New(&influxdb.ClientConfig{
		Host:       *influxDbHost,
		Username:   *influxDbName,
		Password:   *influxDbPassword,
		Database:   *influxDbDatabase,
		HttpClient: nil,
		IsSecure:   false,
		IsUDP:      false,
	})
	if err != nil {
		glog.Fatalf("Failed to initialize InfluxDB client: %s", err)
	}

	// Initialize the buildbot database.
	conf, err := database.ConfigFromFlagsAndMetadata(*local, buildbot.MigrationSteps())
	if err != nil {
		glog.Fatal(err)
	}
	if err := buildbot.InitDB(conf); err != nil {
		glog.Fatal(err)
	}

	// Data generation goroutines.

	// AutoRoll data.
	go autoroll_ingest.LoadAutoRollData(dbClient, *workdir)

	// Buildbot data ingestion.
	go func() {
		// Create the Git repo.
		skiaRepo, err := gitinfo.CloneOrUpdate(SKIA_REPO, path.Join(*workdir, "buildbot_git", "skia"), true)
		if err != nil {
			glog.Fatal(err)
		}
		// Ingest data in a loop.
		for _ = range time.Tick(30 * time.Second) {
			skiaRepo.Update(true, true)
			glog.Info("Ingesting builds.")
			if err := buildbot.IngestNewBuilds(skiaRepo); err != nil {
				glog.Errorf("Failed to ingest new builds: %v", err)
			}
		}
	}()

	// Measure buildbot data ingestion progress.
	totalGuage := metrics.GetOrRegisterGauge("buildbot.builds.total", metrics.DefaultRegistry)
	ingestGuage := metrics.GetOrRegisterGauge("buildbot.builds.ingested", metrics.DefaultRegistry)
	go func() {
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			totalBuilds, err := buildbot.NumTotalBuilds()
			if err != nil {
				glog.Error(err)
				continue
			}
			ingestedBuilds, err := buildbot.NumIngestedBuilds()
			if err != nil {
				glog.Error(err)
				continue
			}
			totalGuage.Update(int64(totalBuilds))
			ingestGuage.Update(int64(ingestedBuilds))
		}
	}()

	// Buildslave uptime.
	go func() {
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			glog.Info("Loading buildslave data.")
			slaves, err := buildbot.GetBuildSlaves()
			if err != nil {
				glog.Error(err)
				continue
			}
			for _, m := range slaves {
				for _, s := range m {
					v := int64(0)
					if s.Connected {
						v = int64(1)
					}
					metric := fmt.Sprintf("buildbot.buildslaves.%s.connected", fixName(s.Name))
					metrics.GetOrRegisterGauge(metric, metrics.DefaultRegistry).Update(v)
				}
			}
		}
	}()

	// Average duration of buildsteps over a time period.
	go func() {
		period := 24 * time.Hour
		type stepData struct {
			Name     string  `db:"name"`
			Duration float64 `db:"duration"`
		}
		stmt, err := buildbot.DB.Preparex(fmt.Sprintf("SELECT name, AVG(finished-started) AS duration FROM %s WHERE started > ? AND finished > started GROUP BY name ORDER BY duration;", buildbot.TABLE_BUILD_STEPS))
		if err != nil {
			glog.Fatalf("Failed to prepare buildbot database query: %v", err)
		}
		defer stmt.Close()
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			glog.Info("Loading buildstep duration data.")
			t := time.Now().UTC().Add(-period).Unix()
			steps := []stepData{}
			if err := stmt.Select(&steps, t); err != nil {
				glog.Error(err)
				continue
			}
			for _, s := range steps {
				v := int64(s.Duration * float64(time.Millisecond))
				metric := fmt.Sprintf("buildbot.buildsteps.%s.duration", fixName(s.Name))
				metrics.GetOrRegisterGauge(metric, metrics.DefaultRegistry).Update(v)
			}
		}
	}()

	// Average duration of builds over a time period.
	go func() {
		period := 24 * time.Hour
		type buildData struct {
			Builder  string  `db:"builder"`
			Duration float64 `db:"duration"`
		}
		stmt, err := buildbot.DB.Preparex(fmt.Sprintf("SELECT builder, AVG(finished-started) AS duration FROM %s WHERE started > ? AND finished > started GROUP BY builder ORDER BY duration;", buildbot.TABLE_BUILDS))
		if err != nil {
			glog.Fatalf("Failed to prepare buildbot database query: %v", err)
		}
		defer stmt.Close()
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			glog.Info("Loading build duration data.")
			t := time.Now().UTC().Add(-period).Unix()
			builds := []buildData{}
			if err := stmt.Select(&builds, t); err != nil {
				glog.Error(err)
				continue
			}
			for _, s := range builds {
				v := int64(s.Duration * float64(time.Millisecond))
				metric := fmt.Sprintf("buildbot.builds.%s.duration", fixName(s.Builder))
				metrics.GetOrRegisterGauge(metric, metrics.DefaultRegistry).Update(v)
			}
		}
	}()

	// Average build step time broken down by builder.
	go func() {
		period := 24 * time.Hour
		type stepData struct {
			Builder  string  `db:"builder"`
			StepName string  `db:"stepName"`
			Duration float64 `db:"duration"`
		}
		stmt, err := buildbot.DB.Preparex(fmt.Sprintf("SELECT b.builder as builder, s.name as stepName, AVG(s.finished-s.started) AS duration FROM %s s INNER JOIN %s b ON (s.buildId = b.id) WHERE s.started > ? AND s.finished > s.started GROUP BY b.builder, s.name ORDER BY b.builder, duration;", buildbot.TABLE_BUILD_STEPS, buildbot.TABLE_BUILDS))
		if err != nil {
			glog.Fatalf("Failed to prepare buildbot database query: %v", err)
		}
		defer stmt.Close()
		for _ = range time.Tick(common.SAMPLE_PERIOD) {
			glog.Info("Loading per-builder buildstep duration data.")
			t := time.Now().UTC().Add(-period).Unix()
			steps := []stepData{}
			if err := stmt.Select(&steps, t); err != nil {
				glog.Error(err)
				continue
			}
			for _, s := range steps {
				v := int64(s.Duration * float64(time.Millisecond))
				metric := fmt.Sprintf("buildbot.buildstepsbybuilder.%s.%s.duration", s.Builder, s.StepName)
				metrics.GetOrRegisterGauge(metric, metrics.DefaultRegistry).Update(v)
			}
		}
	}()

	// Wait while the above goroutines generate data.
	select {}
}
