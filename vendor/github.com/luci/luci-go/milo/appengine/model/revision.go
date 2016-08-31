// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"github.com/luci/gae/service/datastore"
	"golang.org/x/net/context"
)

// Repository represents a repository that a Revision belongs to.
type Repository struct {
	datastore.Key
}

// GetRepository returns the repository object for a given repository URL.
// TODO(martiniss): convert this to luci project name by 2016-01
func GetRepository(c context.Context, repositoryURL string) *Repository {
	ds := datastore.Get(c)
	return &Repository{
		*ds.NewKey("Repository", repositoryURL, 0, nil),
	}
}

// GetRevision returns the corresponding Revision object for a particular
// revision hash in this repository.
func (r *Repository) GetRevision(c context.Context, digest string) (*Revision, error) {
	ds := datastore.Get(c)

	rev := &Revision{
		Digest:     digest,
		Repository: &r.Key,
	}

	err := ds.Get(rev)
	return rev, err
}

// RevisionMetadata is the metadata associated with a particular Revision.
type RevisionMetadata struct {
	// Message is the commit message for a Revision.
	Message string
}

// Revision is a immutable reference to a version of the code in a given
// repository at a given point in time.
//
// Note that currently revisions have no notion of branch. This will need to
// change once we support Chromium.
type Revision struct {
	// Digest is the content hash which uniquely identifies this Revision, in the
	// context of its repository.
	Digest string `gae:"$id"`

	// Repository is the repository this Revision is associated with. See the
	// Repository struct for more info.
	Repository *datastore.Key `gae:"$parent"`

	// Metadata is any metadata (commit message, files changed, etc.)
	// associated with this Revision.
	Metadata RevisionMetadata `gae:",noindex"`

	// Committer is the email of the user who committed this change.
	Committer string

	// Generation is the generation number of this Revision. This is used to sort
	// commits into a roughly time-based order, without using timestamps.
	// See http://www.spinics.net/lists/git/msg161165.html for background.
	Generation int
}
