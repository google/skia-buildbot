// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/luci/gae/service/datastore"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/milo/appengine/model"
	"golang.org/x/net/context"
)

// gitLogLine includes all the info we need from git log.
type gitLogLine struct {
	// digest is the git commit hash.
	digest string

	// parents is a list of parent git hashes.
	parents []string

	// author is the revision author.
	author string

	// epochTime is the revision commit time.
	epochSeconds int64

	// body is the git revision body text.
	body string
}

// getRevision returns a model.Revision using fields common to model.Revision and gitLogLine,
// and the specified generation number.
func (g *gitLogLine) getRevision(generation int) *model.Revision {
	return &model.Revision{
		Digest:     g.digest,
		Metadata:   model.RevisionMetadata{Message: g.body},
		Committer:  g.author,
		Generation: generation,
	}
}

// gitItemFromLine takes the raw string from git log output and returns a gitLogLine or an error
// if there was a problem parsing the string. The input should be a line extracted from git log
// with the format "format:'%H,%P,%ae,%ct,%b'".
func gitItemFromLine(line string) (*gitLogLine, error) {
	parts := strings.SplitN(line, ",", 5)
	if len(parts) != 5 {
		return nil, fmt.Errorf("failed to parse git log line into 5 comma-separated parts: '%s'", line)
	}
	parents := []string{}
	if len(parts[1]) > 0 {
		parents = strings.Split(parts[1], " ")
	}
	epochSecs, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, fmt.Errorf("failed to parse epoch seconds from input: '%s'", parts[3])
	}
	return &gitLogLine{
		digest:       parts[0],
		parents:      parents,
		author:       parts[2],
		epochSeconds: int64(epochSecs),
		body:         parts[4],
	}, nil
}

// GetRevisions takes the full output of:
// git log --topo-order --reverse -z --format=format:'%H,%P,%ae,%ct,%b' and returns a slice of
// model.Revision structs with generation numbers populated. The Repository field is not set in
// the structs since it's a datastore.Key and requires a GAE context associated with the actual
// instance. Callers should set the key explicitly after this returns.
func GetRevisions(contents string) ([]*model.Revision, error) {
	revisionMap := make(map[string]*model.Revision)
	revisionList := make([]*model.Revision, 0, 1000)
	for _, line := range strings.Split(contents, "\x00") {
		gitItem, err := gitItemFromLine(line)
		if err != nil {
			return nil, err
		}

		// This is the first revision in a branch so just set generation to 0.
		if len(gitItem.parents) == 0 {
			revisionMap[gitItem.digest] = gitItem.getRevision(0)
			revisionList = append(revisionList, revisionMap[gitItem.digest])
			continue
		}

		// Calculate the generation number with max(parent generation numbers) + 1.
		var max = -1
		for _, parentDigest := range gitItem.parents {
			parentRev, ok := revisionMap[parentDigest]
			if !ok {
				// This shouldn't happen if --topo-order --reverse were specified correctly.
				return nil, fmt.Errorf("missing parent revision %s for %s", parentDigest, gitItem.digest)
			}
			if parentRev.Generation > max {
				max = parentRev.Generation
			}
		}
		revisionMap[gitItem.digest] = gitItem.getRevision(max + 1)
		revisionList = append(revisionList, revisionMap[gitItem.digest])
	}
	return revisionList, nil
}

// SaveRevisions saves the given Revision entities in batches of 100.
// TODO(estaab): Parallelize this and make it a gae filter.
func SaveRevisions(ctx context.Context, revisions []*model.Revision) error {
	ds := datastore.Get(ctx)
	for lower := 0; lower < len(revisions); lower += 100 {
		upper := lower + 100
		if len(revisions) < upper {
			upper = len(revisions)
		}
		log.Infof(ctx, "Writing revisions [%d, %d) to datastore.", lower, upper)
		if err := ds.Put(revisions[lower:upper]); err != nil {
			return fmt.Errorf("%s", err)
		}
	}
	return nil
}
