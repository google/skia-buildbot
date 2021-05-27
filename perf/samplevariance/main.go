// See README.md
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/query"
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
	workerPoolSize = 64
)

// flags
var (
	prefix = flag.String("prefix", "", "GCS location to search for files. E.g. gs://skia-perf/nano-json-v1/2021/05/23/02/. If not supplied then all the files from yesterday are queried.")
	out    = flag.String("out", "", "Output filename. If not supplied then CSV file is written to stdout.")
	filter = flag.String("filter", "source_type=skp&sub_result=min_ms", "A query to filter the traces.")
	top    = flag.Int("top", 100, "The top number of CSV rows to report. Set to -1 to return all of them.")
)

// sampleInfo is the information we calculate for each test run.
type sampleInfo struct {
	traceid string
	median  float64
	min     float64
	ratio   float64 // ratio = median/min.
}

// sampleInfoSlice is a utility type for sorting slices of sampleInfo by
// descending ratio.
type sampleInfoSlice []sampleInfo

func (p sampleInfoSlice) Len() int           { return len(p) }
func (p sampleInfoSlice) Less(i, j int) bool { return p[i].ratio > p[j].ratio }
func (p sampleInfoSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func main() {
	ctx, bucket, objectPrefix, traceFilter, w := initialize()
	defer util.Close(w)

	filenames, err := filenamesFromBucketAndObjectPrefix(ctx, bucket, objectPrefix)
	if err != nil {
		sklog.Fatal(err)
	}

	samples, err := samplesFromFilenames(ctx, bucket, traceFilter, filenames)
	if err != nil {
		sklog.Fatal(err)
	}

	sort.Sort(sampleInfoSlice(samples))

	if err := writeCSV(samples, *top, w); err != nil {
		sklog.Fatal(err)
	}
	util.Close(w)
}

// initialize parses all the flags and constructs the objects we need from them.
func initialize() (context.Context, *storage.BucketHandle, string, *query.Query, io.WriteCloser) {
	common.Init()
	ctx := context.Background()

	// Determine bucket and object prefix.
	if *prefix == "" {
		*prefix = "gs://skia-perf/nano-json-v1/" + time.Now().Add(-24*time.Hour).Format("2006/01/02/")
	}
	sklog.Infof("Reading JSON files from: %q", *prefix)
	u, err := url.Parse(*prefix)
	if err != nil {
		sklog.Fatal("Failed to parse the prefix %q: %s", *prefix, err)
	}
	bucketName := u.Host
	objectPrefix := u.Path[1:]
	sklog.Infof("Reading JSON files from bucket: %q  path: %q", bucketName, objectPrefix)

	// Construct traceFilter.
	values, err := url.ParseQuery(*filter)
	if err != nil {
		sklog.Fatal("Failed to parse filter %q: %s", *filter, err)
	}
	traceFilter, err := query.New(values)
	if err != nil {
		sklog.Fatal("Failed to build traceFilter from filter %q: %s", *filter, err)
	}

	// Create GCS client.
	gcsClient, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly))
	if err != nil {
		sklog.Fatal("Failed to create GCS client: %s", err)
	}
	bucket := gcsClient.Bucket(bucketName)

	// Determine where to write the results.
	var w io.WriteCloser = os.Stdout
	if *out != "" {
		w, err = os.Create(*out)
		if err != nil {
			sklog.Fatalf("Failed to create %q", *out, err)
		}
	}

	return ctx, bucket, objectPrefix, traceFilter, w
}

func filenamesFromBucketAndObjectPrefix(ctx context.Context, bucket *storage.BucketHandle, objectPrefix string) ([]string, error) {
	// Populate filenames with all the names of the files in GCS that match the prefix and filter.
	filenames := []string{}

	q := &storage.Query{
		Prefix: objectPrefix,
	}
	if err := q.SetAttrSelection([]string{"Name"}); err != nil { // Only return the Name.
		return nil, skerr.Wrapf(err, "Failed to reduce attributes.")
	}
	it := bucket.Objects(ctx, q)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			sklog.Fatalf("Failed iterating names of files in bucket: %s", err)
		}
		filenames = append(filenames, attrs.Name)
	}
	return filenames, nil
}

func samplesFromFilenames(ctx context.Context, bucket *storage.BucketHandle, traceFilter *query.Query, filenames []string) ([]sampleInfo, error) {
	// Protected by mutex.
	samples := []sampleInfo{}

	// Protects allRatios
	var mutex sync.Mutex

	// gcsFilenameChannel is used to distribute work to the workers.
	gcsFilenameChannel := make(chan string, len(filenames))

	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < workerPoolSize; i++ {
		g.Go(func() error {
			for filename := range gcsFilenameChannel {
				info, err := traceInfoFromFilename(ctx, bucket, traceFilter, filename)
				if err != nil {
					return skerr.Wrap(err)
				}
				mutex.Lock()
				samples = append(samples, info...)
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
	return samples, nil
}

// traceInfoFromFilename returns a slice of traceInfos extracted from a single
// JSON file.
func traceInfoFromFilename(ctx context.Context, bucket *storage.BucketHandle, traceFilter *query.Query, filename string) ([]sampleInfo, error) {
	sklog.Info(filename)
	r, err := bucket.Object(filename).NewReader(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to open GCS file: %q", filename)
	}
	defer util.Close(r)
	benchData, err := format.ParseLegacyFormat(r)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse GCS file: %q", filename)
	}
	ret := []sampleInfo{}

	for traceid, samples := range parser.GetSamplesFromLegacyFormat(benchData) {
		// Filter for the types of traces we are interested in.
		if !traceFilter.Matches(traceid) {
			continue
		}
		sort.Float64s(samples.Values) // Sort so we can find the min.
		values := stats.Sample{Xs: samples.Values}
		median := values.Quantile(0.5)
		min := samples.Values[0]
		ratio := median / min
		ret = append(ret, sampleInfo{
			traceid: traceid,
			median:  median,
			min:     min,
			ratio:   ratio,
		})
	}
	return ret, nil
}

// writeCSV to w, using samples, and truncating to 'top' entries if top != -1.
func writeCSV(samples []sampleInfo, top int, w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"traceid", "min", "median", "ratio"}); err != nil {
		return skerr.Wrapf(err, "Failed to write header.")
	}

	count := len(samples)
	if top > 0 && top < count {
		samples = samples[:top]
	}
	for _, info := range samples {
		if err := cw.Write([]string{
			info.traceid,
			fmt.Sprintf("%f", info.min),
			fmt.Sprintf("%f", info.median),
			fmt.Sprintf("%f", info.ratio),
		}); err != nil {
			return skerr.Wrapf(err, "Failed to write row.")
		}
	}
	cw.Flush()
	return nil
}
