package main

import (
	"flag"
	"strings"
	"time"
)

import (
	"skia.googlesource.com/buildbot.git/perf/go/config"
)

var (
	runEvery     = flag.Duration("run_every", 15*time.Minute, "How often the ingester to pull data from Google Storage.")
	isSingleShot = flag.Bool("single_shot", false, "Run the ingester only once.")
	gsPrefixes   = flag.String("gs_prefix", "micro:stats-json-v2,skps:pics-json-v2", "A comma-separated list of GS directory prefixes and their equivalent datasets, as in micro:stats-json-v2,skps:pics-json-v2")
)

func main() {
	flag.Parse()
	Init()

	prefixes := strings.Split(*gsPrefixes, ",")
	// Unfortunately using a map randomizes the order of execution..
	prefixMappings := map[config.DatasetName]string{}
	for i := range prefixes {
		splitPair := strings.Split(prefixes[i], ":")
		prefixMappings[config.DatasetName(splitPair[0])] = splitPair[1]
	}

	RunIngester(prefixMappings)

	if !*isSingleShot && *runEvery > 0 {
		for _ = range time.Tick(*runEvery) {
			RunIngester(prefixMappings)
		}
	}
}
