// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package services

import (
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

func (b *server) ArchiveStream(c context.Context, req *logdog.ArchiveStreamRequest) (*google.Empty, error) {
	log.Fields{
		"project":       req.Project,
		"id":            req.Id,
		"complete":      req.Complete(),
		"terminalIndex": req.TerminalIndex,
		"logEntryCount": req.LogEntryCount,
		"error":         req.Error,
	}.Infof(c, "Received archival request.")

	id := coordinator.HashID(req.Id)
	if err := id.Normalize(); err != nil {
		return nil, grpcutil.Errf(codes.InvalidArgument, "Invalid ID (%s): %s", id, err)
	}

	// Verify that the request is minimially valid.
	switch {
	case req.IndexUrl == "":
		return nil, grpcutil.Errf(codes.InvalidArgument, "missing required index archive URL")
	case req.StreamUrl == "":
		return nil, grpcutil.Errf(codes.InvalidArgument, "missing required stream archive URL")
	}

	di := ds.Get(c)
	lst := coordinator.NewLogStreamState(di, id)

	// Post the archival results to the Coordinator.
	now := clock.Now(c).UTC()
	var ierr error
	err := di.RunInTransaction(func(c context.Context) error {
		ierr = nil

		// Note that within this transaction, we have two return values:
		// - Non-nil to abort the transaction.
		// - Specific error via "ierr".
		di := ds.Get(c)
		if err := di.Get(lst); err != nil {
			return err
		}

		switch as := lst.ArchivalState(); {
		case as.Archived():
			// Return nil if the log stream is already archived (idempotent).
			log.Warningf(c, "Log stream is already archived.")
			return nil

			// If our log stream is not in in a tasked archival state, we will reject
			// this archive request with FailedPrecondition.
		case as != coordinator.ArchiveTasked:
			log.Fields{
				"state": as,
			}.Errorf(c, "Log stream archival is not tasked.")
			ierr = grpcutil.Errf(codes.FailedPrecondition, "Log stream has not tasked an archival.")
			return ierr
		}

		// If this request contained an error, we will record an empty archival and
		// log a warning.
		if req.Error != "" {
			log.Fields{
				"archiveError": req.Error,
			}.Warningf(c, "Log stream archival indicated error. Archiving empty stream.")

			req.TerminalIndex = -1
			req.LogEntryCount = 0
		}

		// Update archival information. Make sure this actually marks the stream as
		// archived.
		lst.Updated = now
		lst.ArchivedTime = now
		lst.ArchivalKey = nil // No point in wasting datastore space on this.

		if lst.TerminalIndex < 0 {
			// Also set the terminated time.
			lst.TerminatedTime = now
		}
		lst.TerminalIndex = req.TerminalIndex

		lst.ArchiveLogEntryCount = req.LogEntryCount
		lst.ArchiveStreamURL = req.StreamUrl
		lst.ArchiveStreamSize = req.StreamSize
		lst.ArchiveIndexURL = req.IndexUrl
		lst.ArchiveIndexSize = req.IndexSize
		lst.ArchiveDataURL = req.DataUrl
		lst.ArchiveDataSize = req.DataSize

		// Update the log stream.
		if err := di.Put(lst); err != nil {
			log.WithError(err).Errorf(c, "Failed to update log stream.")
			return err
		}

		log.Infof(c, "Successfully marked stream as archived.")
		return nil
	}, nil)
	if ierr != nil {
		log.WithError(ierr).Errorf(c, "Failed to mark stream as archived.")
		return nil, ierr
	}
	if err != nil {
		log.WithError(err).Errorf(c, "Internal error.")
		return nil, grpcutil.Internal
	}

	return &google.Empty{}, nil
}
