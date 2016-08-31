// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package collector

import (
	"bytes"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/common/tsmon/distribution"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/metric"
	tsmon_types "github.com/luci/luci-go/common/tsmon/types"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butlerproto"
	"github.com/luci/luci-go/logdog/common/storage"
	"github.com/luci/luci-go/logdog/common/types"
	"github.com/luci/luci-go/logdog/server/collector/coordinator"
	"golang.org/x/net/context"
)

const (
	// DefaultMaxMessageWorkers is the default number of concurrent worker
	// goroutones to employ for a single message.
	DefaultMaxMessageWorkers = 4
)

var (
	// tsBundles tracks the total number of logpb.ButlerLogBundle entries that
	// have been submitted for collection.
	tsBundles = metric.NewCounter("logdog/collector/bundles",
		"The number of individual log entry bundles that have been ingested.",
		tsmon_types.MetricMetadata{})
	// tsLogs tracks the number of logpb.LogEntry entries that have been
	// written to intermediate storage.
	tsLogs = metric.NewCounter("logdog/collector/logs",
		"The number of individual log entries that have been ingested.",
		tsmon_types.MetricMetadata{})

	// tsBundleSize tracks the size, in bytes, of a given log bundle.
	tsBundleSize = metric.NewCumulativeDistribution("logdog/collector/bundle/size",
		"The size (in bytes) of the bundle.",
		tsmon_types.MetricMetadata{Units: tsmon_types.Bytes},
		distribution.DefaultBucketer)
	// tsBundleEntriesPerBundle tracks the number of ButlerLogBundle.Entry entries
	// in each bundle that have been collected.
	tsBundleEntriesPerBundle = metric.NewCumulativeDistribution("logdog/collector/bundle/entries_per_bundle",
		"The number of log bundle entries per bundle.",
		tsmon_types.MetricMetadata{},
		distribution.DefaultBucketer)

	// tsBundleEntries tracks the total number of ButlerLogBundle.Entry entries
	// that have been collected.
	//
	// The "stream" field is the type of log stream for each tracked bundle entry.
	tsBundleEntries = metric.NewCounter("logdog/collector/bundle/entries",
		"The number of Butler bundle entries pulled.",
		tsmon_types.MetricMetadata{},
		field.String("stream"))
	// tsBundleEntryLogs tracks the number of LogEntry ingested per bundle.
	//
	// The "stream" field is the type of log stream.
	tsBundleEntryLogs = metric.NewCumulativeDistribution("logdog/collector/bundle/entry/logs",
		"The number of log entries per bundle.",
		tsmon_types.MetricMetadata{},
		distribution.DefaultBucketer,
		field.String("stream"))
	tsBundleEntryProcessingTime = metric.NewCumulativeDistribution("logdog/collector/bundle/entry/processing_time_ms",
		"The amount of time in milliseconds that a bundle entry takes to process.",
		tsmon_types.MetricMetadata{Units: tsmon_types.Milliseconds},
		distribution.DefaultBucketer,
		field.String("stream"))
)

// Collector is a stateful object responsible for ingesting LogDog logs,
// registering them with a Coordinator, and stowing them in short-term storage
// for streaming and processing.
//
// A Collector's Close should be called when finished to release any internal
// resources.
type Collector struct {
	// Coordinator is used to interface with the Coordinator client.
	//
	// On production systems, this should wrapped with a caching client (see
	// the stateCache sub-package) to avoid overwhelming the server.
	Coordinator coordinator.Coordinator

	// Storage is the intermediate storage instance to use.
	Storage storage.Storage

	// StreamStateCacheExpire is the maximum amount of time that a cached stream
	// state entry is valid. If zero, DefaultStreamStateCacheExpire will be used.
	StreamStateCacheExpire time.Duration

	// MaxMessageWorkers is the maximum number of concurrent workers to employ
	// for any given message. If <= 0, DefaultMaxMessageWorkers will be applied.
	MaxMessageWorkers int
}

// Process ingests an encoded ButlerLogBundle message, registering it with
// the LogDog Coordinator and stowing it in a temporary Storage for streaming
// retrieval.
//
// If a transient error occurs during ingest, Process will return an error.
// If no error occurred, or if there was an error with the input data, no error
// will be returned.
func (c *Collector) Process(ctx context.Context, msg []byte) error {
	tsBundles.Add(ctx, 1)
	tsBundleSize.Add(ctx, float64(len(msg)))

	pr := butlerproto.Reader{}
	if err := pr.Read(bytes.NewReader(msg)); err != nil {
		log.WithError(err).Errorf(ctx, "Failed to unpack message.")
		return nil
	}
	if pr.Metadata.ProtoVersion != logpb.Version {
		log.Fields{
			"messageProtoVersion": pr.Metadata.ProtoVersion,
			"currentProtoVersion": logpb.Version,
		}.Errorf(ctx, "Unknown protobuf version.")
		return nil
	}
	if pr.Bundle == nil {
		log.Errorf(ctx, "Protocol message did not contain a Butler bundle.")
		return nil
	}

	tsBundleEntriesPerBundle.Add(ctx, float64(len(pr.Bundle.Entries)))
	for i, entry := range pr.Bundle.Entries {
		tsBundleEntries.Add(ctx, 1, streamType(entry.Desc))

		// If we're logging INFO or higher, log the ranges that this bundle
		// represents.
		if log.IsLogging(ctx, log.Info) {
			fields := log.Fields{
				"index":   i,
				"project": pr.Bundle.Project,
				"path":    entry.GetDesc().Path(),
			}
			if entry.Terminal {
				fields["terminalIndex"] = entry.TerminalIndex
			}
			if logs := entry.GetLogs(); len(logs) > 0 {
				fields["logStart"] = logs[0].StreamIndex
				fields["logEnd"] = logs[len(logs)-1].StreamIndex
			}

			fields.Infof(ctx, "Processing log bundle entry.")
		}
	}

	lw := bundleHandler{
		msg: msg,
		md:  pr.Metadata,
		b:   pr.Bundle,
	}

	lw.project = config.ProjectName(lw.b.Project)
	if err := lw.project.Validate(); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"project":    lw.b.Project,
		}.Errorf(ctx, "Failed to validate bundle project name.")
		return errors.New("invalid bundle project name")
	}

	if err := types.StreamName(lw.b.Prefix).Validate(); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"prefix":     lw.b.Prefix,
		}.Errorf(ctx, "Failed to validate bundle prefix.")
		return errors.New("invalid bundle prefix")
	}

	// If there are no entries, there is nothing to do.
	if len(pr.Bundle.Entries) == 0 {
		return nil
	}

	// Handle each bundle entry in parallel. We will use a separate work pool
	// here so that top-level bundle dispatch can't deadlock the processing tasks.
	workers := c.MaxMessageWorkers
	if workers <= 0 {
		workers = DefaultMaxMessageWorkers
	}
	return parallel.WorkPool(workers, func(taskC chan<- func() error) {
		for _, be := range pr.Bundle.Entries {
			be := be

			taskC <- func() error {
				return c.processLogStream(ctx, &bundleEntryHandler{
					bundleHandler: &lw,
					be:            be,
				})
			}
		}
	})
}

// Close releases any internal resources and blocks pending the completion of
// any outstanding operations. After Close, no new Process calls may be made.
func (c *Collector) Close() {
}

// bundleHandler is a cumulative set of read-only state passed around by
// value for log processing.
type bundleHandler struct {
	// msg is the original message bytes.
	msg []byte
	// md is the metadata associated with the overall message.
	md *logpb.ButlerMetadata
	// b is the Butler bundle.
	b *logpb.ButlerLogBundle

	// project is the validated project name.
	project config.ProjectName
}

type bundleEntryHandler struct {
	*bundleHandler

	// be is the Bundle entry.
	be *logpb.ButlerLogBundle_Entry
	// path is the constructed path of the stream being processed.
	path types.StreamPath
}

// processLogStream processes an individual set of log messages belonging to the
// same log stream.
func (c *Collector) processLogStream(ctx context.Context, h *bundleEntryHandler) error {
	streamTypeField := streamType(h.be.Desc)
	startTime := clock.Now(ctx)
	defer func() {
		duration := clock.Now(ctx).Sub(startTime)

		// We track processing time in milliseconds.
		tsBundleEntryProcessingTime.Add(ctx, duration.Seconds()*1000, streamTypeField)
	}()

	// If this bundle has neither log entries nor a terminal index, it is junk and
	// must be discarded.
	//
	// This is more important than a basic optimization, as it enforces that no
	// zero-entry log streams can be ingested. Either some entries exist, or there
	// is a promise of a terminal entry.
	if len(h.be.Logs) == 0 && !h.be.Terminal {
		log.Warningf(ctx, "Bundle entry is non-terminal and contains no logs; discarding.")
		return nil
	}

	secret := types.PrefixSecret(h.b.Secret)
	if err := secret.Validate(); err != nil {
		log.Fields{
			log.ErrorKey:   err,
			"secretLength": len(secret),
		}.Errorf(ctx, "Failed to validate prefix secret.")
		return errors.New("invalid prefix secret")
	}

	// If the descriptor has a Prefix, it must match the bundle's Prefix.
	if p := h.be.Desc.Prefix; p != "" {
		if p != h.b.Prefix {
			log.Fields{
				"bundlePrefix":      h.b.Prefix,
				"bundleEntryPrefix": p,
			}.Errorf(ctx, "Bundle prefix does not match entry prefix.")
			return errors.New("mismatched bundle and entry prefixes")
		}
	} else {
		// Fill in the bundle's Prefix.
		h.be.Desc.Prefix = h.b.Prefix
	}

	if err := h.be.Desc.Validate(true); err != nil {
		log.WithError(err).Errorf(ctx, "Invalid log stream descriptor.")
		return err
	}
	descBytes, err := proto.Marshal(h.be.Desc)
	if err != nil {
		log.WithError(err).Errorf(ctx, "Failed to marshal descriptor.")
		return err
	}

	h.path = types.StreamName(h.be.Desc.Prefix).Join(types.StreamName(h.be.Desc.Name))
	ctx = log.SetFields(ctx, log.Fields{
		"project": h.project,
		"path":    h.path,
	})

	// Confirm that the log entries are valid and contiguous. Serialize the log
	// entries for ingest as we validate them.
	var logData [][]byte
	var blockIndex uint64
	if logs := h.be.Logs; len(logs) > 0 {
		logData = make([][]byte, len(logs))
		blockIndex = logs[0].StreamIndex

		for i, le := range logs {
			// Validate this log entry.
			if err := le.Validate(h.be.Desc); err != nil {
				log.Fields{
					log.ErrorKey: err,
					"index":      le.StreamIndex,
				}.Warningf(ctx, "Discarding invalid log entry.")
				return errors.New("invalid log entry")
			}

			// Validate that this entry is contiguous.
			if le.StreamIndex != blockIndex+uint64(i) {
				log.Fields{
					"index":    i,
					"expected": (blockIndex + uint64(i)),
					"actual":   le.StreamIndex,
				}.Errorf(ctx, "Non-contiguous log entry block in stream.")
				return errors.New("non-contiguous log entry block")
			}

			var err error
			logData[i], err = proto.Marshal(le)
			if err != nil {
				log.Fields{
					log.ErrorKey: err,
					"index":      le.StreamIndex,
				}.Errorf(ctx, "Failed to marshal log entry.")
				return errors.New("failed to marshal log entries")
			}
		}
	}

	// Fetch our cached/remote state. This will replace our state object with the
	// fetched state, so any future calls will need to re-set the Secret value.
	// TODO: Use timeout?
	registerReq := coordinator.LogStreamState{
		Project:       h.project,
		Path:          h.path,
		Secret:        secret,
		ProtoVersion:  h.md.ProtoVersion,
		TerminalIndex: -1,
	}
	if h.be.Terminal {
		registerReq.TerminalIndex = types.MessageIndex(h.be.TerminalIndex)
	}
	state, err := c.Coordinator.RegisterStream(ctx, &registerReq, descBytes)
	if err != nil {
		log.WithError(err).Errorf(ctx, "Failed to get/register current stream state.")
		return err
	}

	// Does the log stream's secret match the expected secret?
	if !bytes.Equal([]byte(secret), []byte(state.Secret)) {
		log.Errorf(log.SetFields(ctx, log.Fields{
			"secret":         secret,
			"expectedSecret": state.Secret,
		}), "Log entry has incorrect secret.")
		return nil
	}

	if state.Archived {
		log.Infof(ctx, "Skipping message bundle for archived stream.")
		return nil
	}
	if state.Purged {
		log.Infof(ctx, "Skipping message bundle for purged stream.")
		return nil
	}

	// Update our terminal index if we have one.
	//
	// Note that even if our cached value is marked terminal, we could have failed
	// to push the terminal index to the Coordinator, so we will not refrain from
	// pushing every terminal index encountered regardless of cache state.
	if h.be.Terminal {
		tidx := types.MessageIndex(h.be.TerminalIndex)
		log.Fields{
			"value": tidx,
		}.Debugf(ctx, "Bundle includes a terminal index.")

		if state.TerminalIndex < 0 {
			state.TerminalIndex = tidx
		} else if state.TerminalIndex != tidx {
			log.Fields{
				"cachedIndex": state.TerminalIndex,
				"bundleIndex": tidx,
			}.Warningf(ctx, "Cached terminal index disagrees with state.")
		}
	}

	// Perform stream processing operations. We can do these operations in
	// parallel.
	return parallel.FanOutIn(func(taskC chan<- func() error) {
		// Store log data, if any was provided. It has already been validated.
		if len(logData) > 0 {
			taskC <- func() error {
				// Post the log to storage.
				err = c.Storage.Put(storage.PutRequest{
					Project: h.project,
					Path:    h.path,
					Index:   types.MessageIndex(blockIndex),
					Values:  logData,
				})

				// If the log entry already exists, consider the "put" successful.
				// Storage will return a transient error if one occurred.
				if err != nil && err != storage.ErrExists {
					log.Fields{
						log.ErrorKey: err,
						"blockIndex": blockIndex,
					}.Errorf(ctx, "Failed to load log entry into Storage.")
					return err
				}

				tsLogs.Add(ctx, int64(len(logData)), streamTypeField)
				return nil
			}
		}

		// If our bundle entry is terminal, we have an additional task of reporting
		// this to the Coordinator.
		if h.be.Terminal {
			taskC <- func() error {
				// Sentinel task: Update the terminal bundle state.
				treq := coordinator.TerminateRequest{
					Project:       state.Project,
					Path:          state.Path,
					ID:            state.ID,
					Secret:        state.Secret,
					TerminalIndex: types.MessageIndex(h.be.TerminalIndex),
				}

				log.Fields{
					"terminalIndex": state.TerminalIndex,
				}.Infof(ctx, "Received terminal log; updating Coordinator state.")
				if err := c.Coordinator.TerminateStream(ctx, &treq); err != nil {
					log.WithError(err).Errorf(ctx, "Failed to set stream terminal index.")
					return err
				}
				return nil
			}
		}
	})
}

func streamType(desc *logpb.LogStreamDescriptor) string {
	if desc == nil {
		return "UNKNOWN"
	}
	return desc.StreamType.String()
}
