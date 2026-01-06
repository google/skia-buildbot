package dfiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/progress"
	"golang.org/x/sync/singleflight"
)

// DfProvider is a struct used to manage a cache of dataframes.
type DfProvider struct {
	dfCache map[string]*dataframe.DataFrame
	mutex   sync.RWMutex
	group   singleflight.Group
}

// NewDfProvider returns a new instance of the DfProvider.
func NewDfProvider() *DfProvider {
	return &DfProvider{
		dfCache: map[string]*dataframe.DataFrame{},
	}
}

// GetDataFrame returns a dataframe instance for the provided query and time range.
func (d *DfProvider) GetDataFrame(ctx context.Context, dfBuilder dataframe.DataFrameBuilder, query *query.Query, end time.Time, n int32, progress progress.Progress) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "dfiterProvider.GetDataFrame")
	defer span.End()
	key := key(query, end, n)

	df := d.readFromCache(key)
	if df != nil {
		sklog.Infof("Dataframe found in cache for key: %s", key)
		return df, nil
	}

	// The single flight group manages multiple threads processing keys in parallel.
	// The group ensures that only one thread is executing per key. If a thread is
	// already running on a key, the other thread waits and the same result is returned.
	v, err, _ := d.group.Do(key, func() (interface{}, error) {
		df, err := dfBuilder.NewNFromQuery(ctx, end, query, n, progress)
		if err != nil {
			return nil, err
		}

		d.addToCache(key, df)
		return df, nil
	})

	if err != nil {
		return nil, err
	}

	return v.(*dataframe.DataFrame), nil
}

// readFromCache reads the given key from the cache.
func (d *DfProvider) readFromCache(key string) *dataframe.DataFrame {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	if df, ok := d.dfCache[key]; ok {
		return df
	}
	return nil
}

// addToCache adds the given key and dataframe pair to the cache.
func (d *DfProvider) addToCache(key string, df *dataframe.DataFrame) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.dfCache[key] = df
}

// key generates a key for the cache.
func key(query *query.Query, end time.Time, n int32) string {
	return fmt.Sprintf("%s_%s_%d", query.KeyValueString(), end.String(), n)
}
