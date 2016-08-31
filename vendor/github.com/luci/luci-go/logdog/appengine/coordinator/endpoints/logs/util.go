// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logs

import (
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
)

func buildLogStreamState(ls *coordinator.LogStream, lst *coordinator.LogStreamState) *logdog.LogStreamState {
	lss := logdog.LogStreamState{
		ProtoVersion:  ls.ProtoVersion,
		Created:       google.NewTimestamp(ls.Created),
		TerminalIndex: lst.TerminalIndex,
		Purged:        ls.Purged,
	}

	if ast := lst.ArchivalState(); ast.Archived() {
		lss.Archive = &logdog.LogStreamState_ArchiveInfo{
			IndexUrl:      lst.ArchiveIndexURL,
			StreamUrl:     lst.ArchiveStreamURL,
			DataUrl:       lst.ArchiveDataURL,
			Complete:      ast == coordinator.ArchivedComplete,
			LogEntryCount: lst.ArchiveLogEntryCount,
		}
	}

	return &lss
}

func isNoSuchEntity(err error) bool {
	return errors.Any(err, func(err error) bool {
		return err == ds.ErrNoSuchEntity
	})
}
