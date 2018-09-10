package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	_ "net/http/pprof"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/ingestcommon"
	"golang.org/x/oauth2"
	storage "google.golang.org/api/storage/v1"
)

// flags
var (
	configFilename = flag.String("config_filename", "default.json5", "Configuration file in TOML format.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

type StorageEvent struct {
	ObjectName string `json:"name"`
	Bucket     string `json:"bucket"`
}

// btPerfProcessor implements the ingestion.Processor interface for perf.
type btPerfProcessor struct {
	store *btts.BigTableTraceStore
	vcs   vcsinfo.VCS
}

// newPerfProcessor implements the ingestion.Constructor signature.
//
// Note that ptracestore.Init() needs to be called before starting ingestion so
// that ptracestore.Default is set correctly.
func newPerfProcessor(ctx context.Context, vcs vcsinfo.VCS, tileSize int32, project, instance, table string, ts oauth2.TokenSource) (*btPerfProcessor, error) {
	store, err := btts.NewBigTableTraceStore(ctx, int32(tileSize), table, project, instance, ts)
	if err != nil {
		return nil, err
	}

	return &btPerfProcessor{
		store: store,
		vcs:   vcs,
	}, nil
}

func getParamSAndValues(b *ingestcommon.BenchData) ([]paramtools.Params, []float32, paramtools.ParamSet) {
	params := []paramtools.Params{}
	values := []float32{}
	ps := paramtools.ParamSet{}
	for testName, allConfigs := range b.Results {
		for configName, result := range allConfigs {
			key := paramtools.Params(b.Key).Dup()
			key["test"] = testName
			key["config"] = configName
			key.Add(paramtools.Params(b.Options))

			// If there is an options map inside the result add it to the params.
			if resultOptions, ok := result["options"]; ok {
				if opts, ok := resultOptions.(map[string]interface{}); ok {
					for k, vi := range opts {
						// Ignore the very long and not useful GL_ values, we can retrieve
						// them later via ptracestore.Details.
						if strings.HasPrefix(k, "GL_") {
							continue
						}
						if s, ok := vi.(string); ok {
							key[k] = s
						}
					}
				}
			}

			for k, vi := range result {
				if k == "options" || k == "samples" {
					continue
				}
				key["sub_result"] = k
				floatVal, ok := vi.(float64)
				if !ok {
					sklog.Errorf("Found a non-float64 in %v", result)
					continue
				}
				params = append(params, key.Dup())
				values = append(values, float32(floatVal))
				ps.AddParams(key)
			}
		}
	}
	return params, values, ps
}

// See ingestion.Processor interface.
func (p *btPerfProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	// This does the worst possible thing, which is to ingest a single file at a time.
	// Really we should batch up files so we update the OPS less frequently.
	resultsFile.TimeStamp()
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}
	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	if err != nil {
		return err
	}

	params, values, paramset := getParamSAndValues(benchData)

	index, err := p.vcs.IndexOf(ctx, benchData.Hash)
	if err != nil {
		return fmt.Errorf("Could not ingest, hash not found %q: %s", benchData.Hash, err)
	}
	tileKey := p.store.TileKey(int32(index))
	ops, err := p.store.UpdateOrderedParamSet(tileKey, paramset)
	if err != nil {
		return fmt.Errorf("Could not ingest, failed to update OPS: %s", err)
	}
	encoded := map[string]float32{}
	for i, p := range params {
		key, err := ops.EncodeParamsAsString(p)
		if err != nil {
			return fmt.Errorf("Could not ingest, failed OPS encoding: %s", err)
		}
		encoded[key] = values[i]
	}
	return p.store.WriteTraces(int32(index), encoded, resultsFile.Name(), resultsFile.MD5())
}

func (p *btPerfProcessor) GetIngestionStore() ingestion.IngestionStore {
	return p.store.IngestionStore()
}

// See ingestion.Processor interface.
func (p *btPerfProcessor) BatchFinished() error { return nil }

func main() {
	common.InitWithMust(
		"perf-ingest",
		common.PrometheusOpt(promPort),
	)

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, bigtable.Scope, storage.DevstorageReadOnlyScope)
	if err != nil {
		sklog.Fatalf("Failed to create TokenSource: %s", err)
	}
	client := auth.ClientFromTokenSource(ts)

	vcs, err := gitinfo.CloneOrUpdate(ctx, "https://skia.googlesource.com/skia", "/tmp/skia_ingest_checkout", true)
	if err != nil {
		sklog.Fatal(err)
	}
	source, err := ingestion.NewGoogleStorageSource("nanobt", "skia-perf", "nano-json-v1", client, nil)
	if err != nil {
		sklog.Fatal(err)
	}

	proc, err := newPerfProcessor(ctx, vcs, 50, "skia-public", "perf-bt", "skia", ts)
	if err != nil {
		sklog.Fatal(err)
	}
	// ingestionStore := proc.GetIngestionStore()

	// Kick off the workers.
	sklog.Infoln("Starting workers.")
	NUM_WORKERS := 50
	rchan := make(chan ingestion.ResultFileLocation, NUM_WORKERS)
	for i := 0; i < NUM_WORKERS; i++ {
		go func(i int) {
			for res := range rchan {
				sklog.Infof("Worker (%d) started on: %s", i, res.Name())
				if err := proc.Process(ctx, res); err != nil {
					sklog.Errorf("Failed to ingest %s: %s", res.Name(), err)
				}
			}
		}(i)
	}

	sklog.Infoln("Starting feeder.")
	// Kick off the feeder.
	go func() {
		now := time.Now()
		sklog.Infoln("Starting polling.")
		files, err := source.Poll(now.Add(-time.Hour*24*365).Unix(), now.Unix())
		if err != nil {
			sklog.Fatal(err)
		}
		sklog.Infoln("Starting to feed poll results.")
		for _, f := range files {
			sklog.Infof("Sending: %s", f.Name())
			rchan <- f
		}
	}()

	// Set up the http handler to indicate ready-ness and start serving.
	http.HandleFunc("/ready", httputils.ReadyHandleFunc)
	log.Fatal(http.ListenAndServe(*port, nil))
}
