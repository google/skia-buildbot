package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingestcommon"
	storage "google.golang.org/api/storage/v1"
)

// flags
var (
	configFilename = flag.String("config_filename", "default.json5", "Configuration file in TOML format.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

// btPerfProcessor implements the ingestion.Processor interface for perf.
type btPerfProcessor struct {
	store *btts.BigTableTraceStore
	vcs   vcsinfo.VCS
}

// newPerfProcessor implements the ingestion.Constructor signature.
//
// Note that ptracestore.Init() needs to be called before starting ingestion so
// that ptracestore.Default is set correctly.
func newPerfProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client, eventBus eventbus.EventBus) (ingestion.Processor, error) {
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, bigtable.Scope, storage.DevstorageReadOnlyScope)
	if err != nil {
		return nil, fmt.Errorf("Failed to create TokenSource: %s", err)
	}

	extra := config.ExtraParams

	tileSize, err := strconv.Atoi(extra["tile_size"])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse tile_size: %s Got: %q", err, extra["tile_size"])
	}
	store, err := btts.NewBigTableTraceStore(ctx, int32(tileSize), extra["table"], extra["project"], extra["instance"], ts)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
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
	ingestion.Register(config.CONSTRUCTOR_NANO_BT, newPerfProcessor)

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, bigtable.Scope, storage.DevstorageReadOnlyScope)
	if err != nil {
		sklog.Fatalf("Failed to create TokenSource: %s", err)
	}
	client := auth.ClientFromTokenSource(ts)
	config, err := sharedconfig.ConfigFromJson5File(*configFilename)
	if err != nil {
		sklog.Fatalf("Unable to read config file %s. Got error: %s", *configFilename, err)
	}
	ingesters, err := ingestion.IngestersFromConfig(ctx, config, client, nil, nil)
	if err != nil {
		sklog.Fatalf("Unable to instantiate ingesters: %s", err)
	}
	for _, ingester := range ingesters {
		if err := ingester.Start(ctx); err != nil {
			sklog.Fatalf("Unable to start ingester: %s", err)
		}
	}
	// Set up the http handler to indicate ready-ness and start serving.
	http.HandleFunc("/ready", httputils.ReadyHandleFunc)
	log.Fatal(http.ListenAndServe(*port, nil))
}
