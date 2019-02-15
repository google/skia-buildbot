package isolate_cache

import (
	"context"
	"fmt"
	"sync"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/api/option"
)

// Cache maintains a cache of IsolatedFiles which is backed by BigTable.
type Cache struct {
	// cache maps RepoState and isolate file name to IsolatedFile.
	cache    map[types.RepoState]map[string]*isolate.IsolatedFile
	cacheMtx sync.RWMutex
}

func New(ctx context.Context, isolateClient *isolate.Client) *Cache {
	client, err := bigtable.NewClient(ctx, btProject, btInstance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Failed to create BigTable client: %s", err)
	}
	table := client.Open(BT_TABLE)
	return &Cache{
		cache: map[types.RepoState]map[string]*isolate.IsolatedFile{},
	}
}
