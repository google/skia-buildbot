// Take as input from stdin a single Perf JSON file and emits a single CSV line
// per trace with the trace name, the mean, the min, the max, and the ratio of
// median/min to stdout.
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/ingest/parser"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	poolSize = 64
)

// flags
var (
	prefix = flag.String("prefix", "", "GCS location to search for files. E.g. gs://skia-perf/nano-json-v1/2021/05/23/02/. If not supplied then all the files from yesterday are queried.")
)

type ratio struct {
	traceid string
	median  float64
	min     float64
	ratio   float64
}

func singleStep(ctx context.Context, filename string) ([]ratio, error) {
	benchData, err := format.ParseLegacyFormat(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	w := csv.NewWriter(os.Stdout)
	for traceid, samples := range parser.GetSamplesFromLegacyFormat(benchData) {
		// Filter for the types of traces we are interested in.
		if samples.Params["source_type"] != "skp" || samples.Params["sub_result"] != "min_ms" {
			continue
		}
		sort.Float64s(samples.Values)
		values := stats.Sample{Xs: samples.Values}
		medianFloat := values.Quantile(0.5)
		median := fmt.Sprintf("%f", medianFloat)
		mean := fmt.Sprintf("%f", stats.Mean(samples.Values))
		min := fmt.Sprintf("%f", samples.Values[0])
		max := fmt.Sprintf("%f", samples.Values[len(samples.Values)-1])
		ratio := fmt.Sprintf("%f", medianFloat/samples.Values[0])
		w.Write([]string{
			// If these columns get changed also update run.sh.
			traceid, mean, median, min, max, ratio,
		})
	}
	w.Flush()
}

func main() {
	common.Init()
	ctx := context.Background()

	if *prefix == "" {
		*prefix = "gs://skia-perf/nano-json-v1/" + time.Now().Add(-24*time.Hour).Format("2006/01/02/")
	}
	u, err := url.Parse(*prefix)
	if err != nil {
		sklog.Fatal(err)
	}

	// Populate with all the names of the files in GCS.
	filenames := []string{}
	gcsClient, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly))
	if err != nil {
		sklog.Fatal(err)
	}
	defer gcsClient.Close()
	q := &storage.Query{Prefix: u.Path}
	q.SetAttrSelection([]string{"Name"})
	it := gcsClient.Bucket(u.Host).Objects(ctx, q)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			sklog.Fatalf("Bucket(%q).Objects: %v", u.Host, err)
		}
		filenames = append(filenames, attrs.Name)
	}

	// Protected by mutex.
	allRatios := []ratio{}

	// Protects allRatios
	var mutex sync.Mutex

	// gcsFilenameChannel is used to distribute work to the workers.
	gcsFilenameChannel := make(chan string, len(filenames))

	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < poolSize; i++ {
		g.Go(func() error {
			for filename := range gcsFilenameChannel {
				ratios, err := singleStep(ctx, filename)
				if err != nil {
					return skerr.Wrap(err)
				}
				mutex.Lock()
				allRatios = append(allRatios, ratios...)
				mutex.Unlock()
			}
			return nil
		})
	}

	// Feed the workers.
	for _, filename := range filenames {
		gcsFilenameChannel <- filename
	}
	close(gcsFilenameChannel)

	if err := g.Wait(); err != nil {
		// Empty the channel.
		for range gcsFilenameChannel {
		}
		sklog.Fatal(err)
	}
}
