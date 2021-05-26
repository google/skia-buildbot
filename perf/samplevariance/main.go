package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
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
	"go.skia.org/infra/go/util"
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
	out    = flag.String("out", "", "Output filename. If not supplied then CSV file is written to stdout.")
)

type traceInfo struct {
	traceid string
	median  float64
	min     float64
	ratio   float64
}

type traceInfoSlice []traceInfo

func (p traceInfoSlice) Len() int           { return len(p) }
func (p traceInfoSlice) Less(i, j int) bool { return p[i].ratio > p[j].ratio }
func (p traceInfoSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func traceInfoFromFilename(ctx context.Context, bucket *storage.BucketHandle, filename string) ([]traceInfo, error) {
	r, err := bucket.Object(filename).NewReader(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(r)
	benchData, err := format.ParseLegacyFormat(r)
	if err != nil {
		log.Fatal(err)
	}
	ret := []traceInfo{}

	for traceid, samples := range parser.GetSamplesFromLegacyFormat(benchData) {
		// Filter for the types of traces we are interested in.
		if samples.Params["source_type"] != "skp" || samples.Params["sub_result"] != "min_ms" {
			continue
		}
		sort.Float64s(samples.Values)
		values := stats.Sample{Xs: samples.Values}
		median := values.Quantile(0.5)
		min := samples.Values[0]
		ratio := median / min
		ret = append(ret, traceInfo{
			traceid: traceid,
			median:  median,
			min:     min,
			ratio:   ratio,
		})
	}
	return ret, nil
}

func main() {
	common.Init()
	ctx := context.Background()

	if *prefix == "" {
		*prefix = "gs://skia-perf/nano-json-v1/" + time.Now().Add(-24*time.Hour).Format("2006/01/02/")
	}
	sklog.Infof("Reading JSON files from: %q", *prefix)
	u, err := url.Parse(*prefix)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Reading JSON files from bucket: %q  path: %q", u.Host, u.Path)

	// Populate with all the names of the files in GCS.
	filenames := []string{}
	gcsClient, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly))
	if err != nil {
		sklog.Fatal(err)
	}
	defer gcsClient.Close()
	q := &storage.Query{
		Prefix: u.Path[1:],
	}
	q.SetAttrSelection([]string{"Name"})
	bucket := gcsClient.Bucket(u.Host)
	it := bucket.Objects(ctx, q)
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
	allTraces := []traceInfo{}

	// Protects allRatios
	var mutex sync.Mutex

	// gcsFilenameChannel is used to distribute work to the workers.
	gcsFilenameChannel := make(chan string, len(filenames))

	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < poolSize; i++ {
		g.Go(func() error {
			for filename := range gcsFilenameChannel {
				ratios, err := traceInfoFromFilename(ctx, bucket, filename)
				if err != nil {
					return skerr.Wrap(err)
				}
				mutex.Lock()
				allTraces = append(allTraces, ratios...)
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
	sort.Sort(traceInfoSlice(allTraces))

	var w io.WriteCloser = os.Stdout
	if *out != "" {
		w, err = os.Create(*out)
		if err != nil {
			sklog.Fatal(err)
		}
		defer util.Close(w)
	}

	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"traceid", "min", "median", "ratio"}); err != nil {
		sklog.Fatal(err)
	}
	for _, info := range allTraces[:100] {
		if err := cw.Write([]string{info.traceid, fmt.Sprintf("%f", info.min), fmt.Sprintf("%f", info.median), fmt.Sprintf("%f", info.ratio)}); err != nil {
			sklog.Fatal(err)
		}
	}
	defer cw.Flush()
}
