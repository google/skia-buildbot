// A command line tool for interrogating a ptracestore.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/ptracestore"
)

const (
	// SAMPLE_SIZE is the number of trace to return from the 'sample' command.
	SAMPLE_SIZE = 10
)

// Command line flags.
var (
	begin          = flag.String("begin", "1w", "Select the commit ids for the range beginning this long ago.")
	end            = flag.String("end", "0s", "Select the commit ids for the range ending this long ago.")
	gitRepoDir     = flag.String("git_repo_dir", "../../../skia", "Directory location for the Skia repo.")
	gitRepoURL     = flag.String("git_repo_url", "https://skia.googlesource.com/skia", "The URL to pass to git clone for the source repository.")
	ptraceStoreDir = flag.String("ptrace_store_dir", "/tmp/ptracestore", "The directory where the ptracestore tiles are stored.")
	queryStr       = flag.String("query", "", "A URL encoded query to filter traces against.")
	verbose        = flag.Bool("verbose", false, "Verbose.")
)

var Usage = func() {
	fmt.Printf(`Usage: ptracequery <command> [OPTIONS]...
Inspect and interrogate a ptracestore.

Only one application can interact with a BoltDB database at one time, so the
Perf application should not be running at the same time as ptracequery.

Commands:

  count     	Return the number of traces stored in the given time range.

            	Flags: --begin --end

  sample    	Get a random sampling of traces in the given time range.

            	Flags: --begin --end

  match       Find parameter values that match a query, formatted as URL query parameters.

            	Flags: --begin --end --query

Examples:

  To count all the traces for the first 6 days of the previous week:

    ptracequery count -begin 1w -end 1d

  To match all traces and return the last three days of values for traces that
  have the test name 'draw_stroke_bezier' but that have an arch that does not
  equal 'x86'.

    ptracequery query --begin=3d --query='test=draw_stroke_bezier&arch=!x86'

Flags:

`)
	flag.PrintDefaults()
}

func progress(step, totalSteps int) {
	sklog.Infof("Progress - %0.2f", 100.0*float32(step)/float32(totalSteps))
}

// _df returns the DataFrame that matches the given query in the range of the
// --begin and --end command line flags.
func _df(vcs vcsinfo.VCS, store ptracestore.PTraceStore, q *query.Query) (*dataframe.DataFrame, error) {
	now := time.Now()
	b, err := human.ParseDuration(*begin)
	if err != nil {
		return nil, fmt.Errorf("Invalid begin value: %s\n", err)
	}
	e, err := human.ParseDuration(*end)
	if err != nil {
		return nil, fmt.Errorf("Invalid begin value: %s\n", err)
	}
	beginTime := now.Add(-b)
	endTime := now.Add(-e)
	if *verbose {
		fmt.Printf("Requesting from %s to %s\n", beginTime, endTime)
	}
	dfBuilder := dataframe.NewDataFrameBuilderFromPTraceStore(vcs, store)
	return dfBuilder.NewFromQueryAndRange(beginTime, endTime, q, progress)
}

func count(vcs vcsinfo.VCS, store ptracestore.PTraceStore) {
	df, err := _df(vcs, store, &query.Query{})
	if err != nil {
		fmt.Printf("Failed to load traces: %s", err)
		return
	}
	fmt.Printf("Count: %d\n", len(df.TraceSet))
}

func match(vcs vcsinfo.VCS, store ptracestore.PTraceStore) {
	u, err := url.ParseQuery(*queryStr)
	if err != nil {
		fmt.Printf("Not a valid URL query %q: %s", *queryStr, err)
	}
	q, err := query.New(u)
	if err != nil {
		fmt.Printf("Not a valid query %q: %s", *queryStr, err)
	}
	df, err := _df(vcs, store, q)
	if err != nil {
		fmt.Printf("Failed to load traces: %s", err)
		return
	}
	for k, v := range df.TraceSet {
		fmt.Printf("%q: %v\n", k, v)
	}
}

func sample(vcs vcsinfo.VCS, store ptracestore.PTraceStore) {
	df, err := _df(vcs, store, &query.Query{})
	if err != nil {
		fmt.Printf("Failed to load traces: %s", err)
		return
	}
	// Use https://en.wikipedia.org/wiki/Reservoir_sampling to pick out our samples.
	i := 0
	type results struct {
		key   string
		value []float32
	}
	res := make([]*results, SAMPLE_SIZE)
	for k, v := range df.TraceSet {
		if i < SAMPLE_SIZE {
			res[i] = &results{key: k, value: v}
		} else {
			r := rand.Intn(i + 1)
			if r < SAMPLE_SIZE {
				res[r] = &results{key: k, value: v}
			}
		}
		i++
	}

	for _, r := range res {
		fmt.Printf("%q: %v\n", r.key, r.value)
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	flag.Usage = Usage
	// Grab the first argument off of os.Args, the command, before we call flag.Parse.
	if len(os.Args) < 2 {
		Usage()
		return
	}
	cmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	flag.Parse()

	git, err := gitinfo.CloneOrUpdate(context.Background(), *gitRepoURL, *gitRepoDir, false)
	if err != nil {
		sklog.Fatal(err)
	}

	ptracestore.Init(*ptraceStoreDir)

	switch cmd {
	case "count":
		count(git, ptracestore.Default)
	case "sample":
		sample(git, ptracestore.Default)
	case "query":
		match(git, ptracestore.Default)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		Usage()
	}
}
