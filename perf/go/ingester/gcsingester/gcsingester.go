// Package gcsingester implements ingester.Source from Google Cloud Storage.
package gcsingester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingestcommon"
	"go.skia.org/infra/perf/go/ingester"
	"go.skia.org/infra/perf/go/types"
	"google.golang.org/api/option"
)

const (
	// mapParallelReceives is the number of Go routines we want to run. Determined experimentally.
	mapParallelReceives = 1
)

// GitHashToCommitNumber is a func that converts a Git hash to a
// types.CommitNumber.
//
// types.BadCommitNumber is returned if the hash can't be found or an error occurred.
type GitHashToCommitNumber func(hash string) types.CommitNumber

// GCSIngesterSource implements ingester.Source for Google Cloud Storage.
type GCSIngesterSource struct {
	// nackCounter is the number files we weren't able to ingest.
	nackCounter metrics2.Counter

	// ackCounter is the number files we were able to ingest.
	ackCounter metrics2.Counter

	// config if the InstanceConfig we are ingesting files for.
	config *config.InstanceConfig

	gitHashToCommitNumber GitHashToCommitNumber

	ch chan ingester.File

	// local is true if running locally.
	local bool
}

// pubSubEvent is used to deserialize the PubSub data.
//
// The PubSub event data is a JSON serialized storage.ObjectAttrs object.
// See https://cloud.google.com/storage/docs/pubsub-notifications#payload
type pubSubEvent struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
}

// New returns a instance of GCSIngesterSource.
func New(config *config.InstanceConfig, gitHashToCommitNumber GitHashToCommitNumber, local bool) (*GCSIngesterSource, error) {
	return &GCSIngesterSource{
		nackCounter:           metrics2.GetCounter("nack", nil),
		ackCounter:            metrics2.GetCounter("ack", nil),
		config:                config,
		gitHashToCommitNumber: gitHashToCommitNumber,
		local:                 local,
	}, nil
}

// getLegacyParamsAndValues returns two parallel slices, each slice contains the
// params and then the float for a single value of a trace. It also returns the
// consolidated ParamSet built from all the Params.
func getLegacyParamsAndValues(b *ingestcommon.BenchData) ([]paramtools.Params, []float64) {
	params := []paramtools.Params{}
	values := []float64{}
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
				values = append(values, floatVal)
			}
		}
	}
	return params, values
}

func (g *GCSIngesterSource) extractFromLegacyFile(r io.Reader, filename string) ([]paramtools.Params, []float64, string, error) {

	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	if err != nil {
		return nil, nil, "", err
	}

	branch, ok := benchData.Key["branch"]
	if ok {
		if len(g.config.IngestionConfig.Branches) > 0 {
			if !util.In(branch, g.config.IngestionConfig.Branches) {
				return nil, nil, "", skerr.Fmt("Data ")
			}
		}
	} else {
		sklog.Infof("No branch name.")
	}

	params, values := getLegacyParamsAndValues(benchData)
	if len(params) == 0 {
		metrics2.GetCounter("perf_ingest_no_data_in_file", map[string]string{"branch": branch}).Inc(1)
		sklog.Infof("No data in: %q", filename)
	}

	return params, values, benchData.Hash, nil
}

func (g *GCSIngesterSource) extractFromFile(r io.Reader, filename string) ([]paramtools.Params, []float64, string, error) {

	var file ingester.FileFormat
	if err := json.NewDecoder(r).Decode(&file); err != nil {
		return nil, nil, "", skerr.Wrap(err)
	}

	// If the file.Version is not correct then it may be in the legacy format,
	// which we parse here.
	if file.Version != ingester.FileFormatVersion {
		return nil, nil, "", skerr.Fmt("File is not in ingester.FileFormat format.")
	}

	params := make([]paramtools.Params, len(file.Results))
	values := make([]float64, len(file.Results))
	common := paramtools.Params(file.Common)
	for i, r := range file.Results {
		key := common.Copy()
		key.Add(r.Key)
		params[i] = key
		values[i] = r.Value
	}
	return params, values, file.Hash, nil
}

func (g *GCSIngesterSource) parseSourceFile(r io.Reader, filename string) ([]paramtools.Params, []float64, string, error) {
	// Assume data is in the new format first.
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, "", skerr.Wrap(err)
	}
	buf := bytes.NewReader(b)

	// If we fail to load data from the file then try parsing it in the legacy format.
	params, values, hash, err := g.extractFromFile(buf, filename)
	if err != nil {
		buf.Seek(0, 0)
		params, values, hash, err = g.extractFromLegacyFile(buf, filename)
	}
	if err != nil {
		return nil, nil, "", err
	}

	// Don't do any more work if there's no data to ingest.
	if len(params) == 0 {
		return nil, nil, "", skerr.Fmt("No data in file: %q", filename)
	}
	return params, values, hash, nil
}

// processSingleFile parses the contents of a single JSON file and writes the values into BigTable.
//
// If 'branches' is not empty then restrict to ingesting just the branches in the slice.
func (g *GCSIngesterSource) processSingleFile(ctx context.Context, filename string, r io.Reader, timestamp time.Time, branches []string) error {
	params, values, hash, err := g.parseSourceFile(r, filename)
	if err != nil {
		return err
	}
	commitNumber := g.gitHashToCommitNumber(hash)
	if commitNumber == types.BadCommitNumber {
		return skerr.Fmt("Could not ingest, hash not found %q", hash)
	}
	g.ch <- ingester.File{
		CommitNumber: commitNumber,
		Params:       params,
		Values:       values,
		Filename:     filename,
		Timestamp:    timestamp,
	}
	return nil
}

// Start implements ingester.Source.
func (g *GCSIngesterSource) Start(ctx context.Context) (<-chan ingester.File, error) {

	hostname, err := os.Hostname()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ts, err := auth.NewDefaultTokenSource(g.local, storage.ScopeReadOnly, pubsub.ScopePubSub)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	client := httputils.DefaultClientConfig().WithTokenSource(ts).WithoutRetries().Client()
	gcsClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	pubSubClient, err := pubsub.NewClient(ctx, g.config.IngestionConfig.Project, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// When running in production we have every instance use the same topic name so that
	// they load-balance pulling items from the topic.
	subName := fmt.Sprintf("%s-%s", g.config.IngestionConfig.Topic, "prod")
	if g.local {
		// When running locally create a new topic for every host.
		subName = fmt.Sprintf("%s-%s", g.config.IngestionConfig.Topic, hostname)
	}
	sub := pubSubClient.Subscription(subName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if !ok {
		sub, err = pubSubClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: pubSubClient.Topic(g.config.IngestionConfig.Topic),
		})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	// How many Go routines should be processing messages?
	sub.ReceiveSettings.MaxOutstandingMessages = mapParallelReceives
	sub.ReceiveSettings.NumGoroutines = mapParallelReceives

	ret := make(chan ingester.File)
	g.ch = ret

	// Process all incoming PubSub requests.
	go func() {
		for {
			// Wait for PubSub events.
			err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
				// Set success to true if we should Ack the PubSub message, otherwise
				// the message will be Nack'd, and PubSub will try to send the message
				// again.
				success := false
				defer func() {
					if success {
						g.ackCounter.Inc(1)
						msg.Ack()
					} else {
						g.nackCounter.Inc(1)
						msg.Nack()
					}
				}()
				// Decode the event, which is a GCS event that a file was written.
				var event pubSubEvent
				if err := json.Unmarshal(msg.Data, &event); err != nil {
					sklog.Error(err)
					return
				}
				// Transaction logs for android_ingest are written to the same bucket,
				// which we should ignore.
				if strings.Contains(event.Name, "/tx_log/") {
					// Ack the file so we don't process it again.
					success = true
					return
				}
				// Load the file.
				obj := gcsClient.Bucket(event.Bucket).Object(event.Name)
				attrs, err := obj.Attrs(ctx)
				if err != nil {
					sklog.Errorf("Failed to retrieve bucket %q object %q: %s", event.Bucket, event.Name, err)
					return
				}
				reader, err := obj.NewReader(ctx)
				if err != nil {
					sklog.Error(err)
					return
				}
				defer util.Close(reader)
				sklog.Infof("Filename: %q", attrs.Name)
				fullName := fmt.Sprintf("gs://%s/%s", event.Bucket, event.Name)

				err = g.processSingleFile(ctx, fullName, reader, attrs.Created, g.config.IngestionConfig.Branches)

				if err != nil {
					sklog.Errorf("Failed to process results: %s", err)
					return
				}
				success = true
			})
			if err != nil {
				sklog.Errorf("Failed receiving pubsub message: %s", err)
			}
		}
	}()

	return ret, nil
}
