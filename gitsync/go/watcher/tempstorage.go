package watcher

import (
	"context"
	"encoding/gob"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

// For the initial ingestion, it's simpler not to deal with branches or commit
// indices until we have all of the commits. tmpGitStore facilitates that by
// simply storing LongCommits without concern for indices or branches.
type tmpGitStore interface {
	// GetAll returns all commits in the tmpGitStore.
	GetAll(context.Context) (map[string]*vcsinfo.LongCommit, error)
	// Put inserts the given commits into the tmpGitStore.
	Put(context.Context, []*vcsinfo.LongCommit) error
	// Delete removes all data stored in the tmpGitStore.
	Delete(context.Context) error
}

// memTmpGitStore is an in-memory implementation of tmpGitStore.
type memTmpGitStore struct {
	commits map[string]*vcsinfo.LongCommit
}

// newMemTmpGitStore returns a tmpGitStore instance which is backed by GCS.
func newMemTmpGitStore() tmpGitStore {
	return &memTmpGitStore{
		commits: map[string]*vcsinfo.LongCommit{},
	}
}

// See documentation for tmpGitStore interface.
func (s *memTmpGitStore) GetAll(_ context.Context) (map[string]*vcsinfo.LongCommit, error) {
	rv := make(map[string]*vcsinfo.LongCommit, len(s.commits))
	for k, v := range s.commits {
		rv[k] = v
	}
	return rv, nil
}

// See documentation for tmpGitStore interface.
func (s *memTmpGitStore) Put(_ context.Context, commits []*vcsinfo.LongCommit) error {
	for _, c := range commits {
		s.commits[c.Hash] = c
	}
	return nil
}

// See documentation for tmpGitStore interface.
func (s *memTmpGitStore) Delete(_ context.Context) error {
	s.commits = map[string]*vcsinfo.LongCommit{}
	return nil
}

// gcsTmpGitStore is an implementation of tmpGitStore which uses GCS.
type gcsTmpGitStore struct {
	mem  tmpGitStore
	c    gcs.GCSClient
	file string
}

// newGCSTmpGitStore returns a tmpGitStore instance which is backed by GCS.
func newGCSTmpGitStore(ctx context.Context, gcsBucket, repoUrl string) (tmpGitStore, error) {
	mem := newMemTmpGitStore()
	s, err := storage.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create storage client.")
	}
	normUrl, err := git.NormalizeURL(repoUrl)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to normalize repo URL %s", repoUrl)
	}
	file := strings.ReplaceAll(normUrl, "/", "_")
	c := gcsclient.New(s, gcsBucket)
	return &gcsTmpGitStore{
		mem:  mem,
		c:    c,
		file: file,
	}, nil
}

// See documentation for tmpGitStore interface.
func (s *gcsTmpGitStore) GetAll(ctx context.Context) (map[string]*vcsinfo.LongCommit, error) {
	r, err := s.c.FileReader(ctx, s.file)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create FileReader")
	}
	defer util.Close(r)
	var rv map[string]*vcsinfo.LongCommit
	if err := gob.NewDecoder(r).Decode(&rv); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode commits")
	}
	s.mem.(*memTmpGitStore).commits = rv
	return rv, nil
}

// See documentation for tmpGitStore interface.
func (s *gcsTmpGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	if err := s.mem.Put(ctx, commits); err != nil {
		skerr.Wrap(err)
	}
	all, err := s.mem.GetAll(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	w := s.c.FileWriter(ctx, s.file, gcs.FileWriteOptions{
		ContentEncoding: "text/plain",
	})
	defer util.Close(w)
	if err := gob.NewEncoder(w).Encode(all); err != nil {
		return skerr.Wrapf(err, "Failed to encode commits")
	}
	return nil
}

// See documentation for tmpGitStore interface.
func (s *gcsTmpGitStore) Delete(ctx context.Context) error {
	if err := s.c.DeleteFile(ctx, s.file); err != nil {
		return skerr.Wrapf(err, "Failed to delete file")
	}
	return s.mem.Delete(ctx)
}
