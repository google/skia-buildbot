package main

// bt_reingester will scan through all the files in a GCS bucket and ingest
// them into the bt_tracestore.
import (
	"flag"
	"time"

	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/sklog"
)

func main() {
	// var (
	// 	btInstance = flag.String("bt_instance", "production", "BigTable instance to use in the project identified by 'project_id'")
	// 	projectID  = flag.String("project_id", "skia-public", "GCP project ID.")
	// 	btTableID  = flag.String("bt_table_id", "", "BigTable table ID for the traces.")

	// 	gitBTTableID = flag.String("git_table_id", "git-repos", "BigTable table ID that has the git data.")
	// 	gitRepoURL   = flag.String("git_repo_url", "", "The URL of the git repo to look up in BigTable.")

	// 	srcBucket  = flag.String("src_bucket", "", "Source bucket to ingest files from.")
	// 	srcRootDir = flag.String("src_root_dir", "", "Source root directory to ingest files in.")
	// )
	flag.Parse()

	bt.EnsureNotEmulator()

	gb, err := gevent.New("skia-public", "gold-chrome-gpu-eventbus-bt", "askdfkjasdkfaskdfsadf")
	if err != nil {
		sklog.Fatalf("no gevents: %s", err)
	}
	gb.PublishStorageEvent(eventbus.NewStorageEvent("skia-gold-chrome-gpu", "dm-json-v1/test.json", time.Now().Unix(), "foo"))

	sklog.Infof("published event")

	time.Sleep(3 * time.Second)
}

// In the early days, there was several invalid entries, because they did not specify
// gitHash. Starting re-ingesting Skia on October 1, 2014 seems to be around when
// the data is correct.
var beginning = time.Date(2014, time.October, 1, 0, 0, 0, 0, time.UTC)
