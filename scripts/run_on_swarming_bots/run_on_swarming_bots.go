package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/swarming"
)

/*
	Run a specified command on all specified GCE instances.
*/

const (
	ISOLATE_DEFAULT_NAMESPACE = "default-gzip"
	ISOLATE_SERVER            = "https://isolateserver.appspot.com"
	TMP_ISOLATE_FILE_NAME     = "script.isolate"
	TMP_ISOLATE_FILE_CONTENTS = `{
  'variables': {
    'command': [
      'python', '%s',
    ],
    'files': [
      '%s',
    ],
  },
}`
)

var (
	dimensions = common.NewMultiStringFlag("dimension", nil, "Colon-separated key/value pair, eg: \"os:Linux\" Dimensions of the bots on which to run. Can specify multiple times.")
	pool       = flag.String("pool", swarming.DIMENSION_POOL_VALUE_SKIA, "Which Swarming pool to use.")
	script     = flag.String("script", "", "Path to a Python script to run.")
	taskName   = flag.String("task_name", "", "Name of the task to run.")
	workdir    = flag.String("workdir", os.TempDir(), "Working directory. Optional, but recommended not to use CWD.")
)

func main() {
	// Setup, parse args.
	defer common.LogPanic()
	common.Init()

	if *script == "" {
		glog.Fatal("--script is required.")
	}
	scriptName := path.Base(*script)
	if *taskName == "" {
		*taskName = fmt.Sprintf("run_%s", scriptName)
	}

	dims, err := swarming.ParseDimensionFlags(dimensions)
	if err != nil {
		glog.Fatalf("Problem parsing dimensions: %s", err)
	}
	dims["pool"] = *pool

	*workdir, err = filepath.Abs(*workdir)
	if err != nil {
		glog.Fatal(err)
	}

	// Authenticated HTTP client.
	oauthCacheFile := path.Join(*workdir, "google_storage_token.data")
	httpClient, err := auth.NewClient(true, oauthCacheFile, swarming.AUTH_SCOPE)
	if err != nil {
		glog.Fatal(err)
	}

	// Swarming API client.
	swarmApi, err := swarming.NewApiClient(httpClient)
	if err != nil {
		glog.Fatal(err)
	}

	// Obtain the list of bots.
	bots, err := swarmApi.ListBots(dims)
	if err != nil {
		glog.Fatal(err)
	}

	swarming, err := swarming.NewSwarmingClient(*workdir)
	if err != nil {
		glog.Fatal(err)
	}

	// Copy the script to the workdir.
	dstScript := path.Join(*workdir, scriptName)
	contents, err := ioutil.ReadFile(*script)
	if err != nil {
		glog.Fatal(err)
	}
	if err := ioutil.WriteFile(dstScript, contents, 0644); err != nil {
		glog.Fatal(err)
	}

	// Create an isolate file.
	isolateFile := path.Join(*workdir, TMP_ISOLATE_FILE_NAME)
	if err := ioutil.WriteFile(isolateFile, []byte(fmt.Sprintf(TMP_ISOLATE_FILE_CONTENTS, scriptName, scriptName)), 0644); err != nil {
		glog.Fatal(err)
	}

	// Upload to isolate server.
	isolated, err := swarming.CreateIsolatedGenJSON(isolateFile, *workdir, "linux", *taskName, map[string]string{}, []string{})
	if err != nil {
		glog.Fatal(err)
	}
	m, err := swarming.BatchArchiveTargets([]string{isolated}, 5*time.Minute)
	if err != nil {
		glog.Fatal(err)
	}
	group := fmt.Sprintf("%s_%s", *taskName, uuid.NewV1())
	tags := map[string]string{
		"group": group,
	}

	// Trigger the task on each bot.
	var wg sync.WaitGroup
	for _, bot := range bots {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			dims := map[string]string{
				"pool": *pool,
				"id":   id,
			}
			if _, err := swarming.TriggerSwarmingTasks(m, dims, tags, 100, 120*time.Minute, 120*time.Minute, 120*time.Minute, false, false); err != nil {
				glog.Fatal(err)
			}
		}(bot.BotId)
	}
	wg.Wait()
	tasksLink := fmt.Sprintf("https://chromium-swarm.appspot.com/tasklist?f=group:%s", group)
	glog.Infof("Triggered Swarming tasks. Visit this link to track progress:\n%s", tasksLink)
}
