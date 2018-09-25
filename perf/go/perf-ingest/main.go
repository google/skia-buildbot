// perf-ingest listens to a PubSub Topic for new files that appear
// in a storage bucket and then ingests those files into BigTable.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingestcommon"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// flags
var (
	configName = flag.String("config_name", "nano", "Name of the perf ingester config to use.")
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port       = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

const (
	// MAX_PARALLEL_RECEIVES is the number of Go routines we want to run. Determined experimentally.
	MAX_PARALLEL_RECEIVES = 8
)

var (
	NonRecoverableError = errors.New("Non-recoverable ingestion error.")
)

func newPerfProcessor(ctx context.Context, vcs vcsinfo.VCS, cfg *config.PerfBigTableConfig, ts oauth2.TokenSource) (*btPerfProcessor, error) {
	store, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, ts, true)
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
			key := paramtools.Params(b.Key).Copy()
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

				key = query.ForceValid(key)
				params = append(params, key.Copy())
				values = append(values, float32(floatVal))
				ps.AddParams(key)
			}
		}
	}
	ps.Normalize()
	return params, values, ps
}

type btPerfProcessor struct {
	store *btts.BigTableTraceStore
	vcs   vcsinfo.VCS
}

func (p *btPerfProcessor) Process(ctx context.Context, name string, r io.Reader, timestamp time.Time) error {
	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	if err != nil {
		sklog.Errorf("Failed to read or parse data: %s", err)
		return NonRecoverableError
	}

	params, values, paramset := getParamSAndValues(benchData)
	index, err := p.vcs.IndexOf(ctx, benchData.Hash)
	if err != nil {
		if err := p.vcs.Update(context.Background(), true, false); err != nil {
			return fmt.Errorf("Could not ingest, failed to pull: %s", err)
		}
		index, err = p.vcs.IndexOf(ctx, benchData.Hash)
		if err != nil {
			sklog.Errorf("Could not ingest, hash not found even after pulling %q: %s", benchData.Hash, err)
			return NonRecoverableError
		}
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
			sklog.Errorf("Could not ingest, failed OPS encoding: %s", err)
			return NonRecoverableError
		}
		encoded[key] = values[i]
	}
	return p.store.WriteTraces(int32(index), encoded, name, timestamp)
}

// Event is used to deserialize the PubSub data.
//
// The PubSub event data is a JSON serialized storage.ObjectAttrs object.
// See https://cloud.google.com/storage/docs/pubsub-notifications#payload
type Event struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

func main() {
	common.InitWithMust(
		"perf-ingest",
		common.PrometheusOpt(promPort),
	)

	// nackCounter is the number files we weren't able to ingest.
	nackCounter := metrics2.GetCounter("nack", nil)

	ctx := context.Background()
	cfg, ok := config.PERF_BIGTABLE_CONFIGS[*configName]
	if !ok {
		sklog.Fatalf("Invalid --config value: %q", *configName)
	}
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatalf("Failed to get hostname: %s", err)
	}
	ts, err := auth.NewDefaultTokenSource(*local, bigtable.Scope, storage.ScopeReadOnly, pubsub.ScopePubSub)
	if err != nil {
		sklog.Fatalf("Failed to create TokenSource: %s", err)
	}

	client := auth.ClientFromTokenSource(ts)
	gcsClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Failed to create GCS client: %s", err)
	}
	pubSubClient, err := pubsub.NewClient(ctx, cfg.Project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}

	// When running in production we have every instance use the same topic name so that
	// they load-balance pulling items from the topic.
	subName := fmt.Sprintf("%s-%s", cfg.Topic, "prod")
	if *local {
		// When running locally create a new topic for every host.
		subName = fmt.Sprintf("%s-%s", cfg.Topic, hostname)
	}
	sub := pubSubClient.Subscription(subName)
	ok, err = sub.Exists(ctx)
	if err != nil {
		sklog.Fatalf("Failed checking subscription existence: %s", err)
	}
	if !ok {
		sub, err = pubSubClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: pubSubClient.Topic(cfg.Topic),
		})
		if err != nil {
			sklog.Fatalf("Failed creating subscription: %s", err)
		}
	}

	// How many Go routines should be processing messages?
	sub.ReceiveSettings.MaxOutstandingMessages = MAX_PARALLEL_RECEIVES
	sub.ReceiveSettings.NumGoroutines = MAX_PARALLEL_RECEIVES

	vcs, err := gitinfo.CloneOrUpdate(ctx, cfg.GitUrl, "/tmp/skia_ingest_checkout", true)
	if err != nil {
		sklog.Fatal(err)
	}

	processor, err := newPerfProcessor(ctx, vcs, cfg, ts)
	if err != nil {
		sklog.Fatal(err)
	}

	// Process all incoming PubSub requests.
	go func() {
		for {
			// Wait for PubSub events.
			err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				success := false
				defer func() {
					if success {
						msg.Ack()
					} else {
						nackCounter.Inc(1)
						msg.Nack()
					}
				}()
				// Decode the event, which is a GCS event that a file was written.
				var event Event
				if err := json.Unmarshal(msg.Data, &event); err != nil {
					sklog.Error(err)
					return
				}
				// Load the file.
				obj := gcsClient.Bucket(event.Bucket).Object(event.Name)
				attrs, err := obj.Attrs(ctx)
				if err != nil {
					sklog.Error(err)
					return
				}
				reader, err := obj.NewReader(ctx)
				if err != nil {
					sklog.Error(err)
					return
				}
				sklog.Info(attrs.Name)
				// Pull data out of file and write it into BigTable.
				err = processor.Process(ctx, attrs.Name, reader, attrs.Created)
				if err := reader.Close(); err != nil {
					sklog.Errorf("Failed to close: %s", err)
				}
				if err == NonRecoverableError {
					success = true
				} else if err != nil {
					sklog.Errorf("Failed to write results: %s", err)
					return
				}
				success = true
			})
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()

	// Set up the http handler to indicate ready-ness and start serving.
	http.HandleFunc("/ready", httputils.ReadyHandleFunc)
	log.Fatal(http.ListenAndServe(*port, nil))
}
