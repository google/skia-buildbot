package gtracestore

import (
	"context"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/vcsinfo"
)

type TraceStore interface {
	Put(ctx context.Context, commit *vcsinfo.LongCommit, entries []*Entry) error
	GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, []int, error)
}

type Entry struct {
	Params map[string]string
	Value  []byte
}

type BTConfig struct {
	ProjectID  string
	InstanceID string
	TableID    string
	VCS        vcsinfo.VCS
	TileSize   int32
	Shards     int32
}

type btTraceStore struct {
	vcs      vcsinfo.VCS
	client   *bigtable.Client
	tileSize int32
	shards   int32
}

func NewBTTraceStore(ctx context.Context, conf *BTConfig) (TraceStore, error) {
	client, err := bigtable.NewClient(ctx, conf.ProjectID, conf.InstanceID)
	if err != nil {
		return nil, err
	}

	ret := &btTraceStore{
		vcs:      conf.VCS,
		client:   client,
		tileSize: conf.TileSize,
		shards:   conf.Shards,
	}
	return ret, nil
}

func (b *btTraceStore) Put(ctx context.Context, commit *vcsinfo.LongCommit, entries []*Entry) error {
	commitIndex, err := b.vcs.IndexOf(ctx, commit.Hash)
	if err != nil {
		return err
	}

	paramSet := paramtools.ParamSet{}
	for _, entry := range entries {
		paramSet.AddParams(entry.Params)
	}

	tileOffset := int32(commitIndex) / b.tileSize
	tileKey := TileKeyFromOffset(tileOffset)
	ops, err := b.updateOrderedParamSet(paramSet)
	if err != nil {
		return err
	}

	return nil
}

func (b *btTraceStore) GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, []int, error) {
	return nil, nil, nil, nil
}

func (b *btTraceStore) updateOrderedParamSet(paramSet paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	return nil, nil
}

type TileWriter struct{}
