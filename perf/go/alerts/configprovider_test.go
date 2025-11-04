package alerts

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockStore struct {
	alerts    []*Alert
	listCount int
	mutex     sync.Mutex
}

func (store *MockStore) ReplaceAll(ctx context.Context, req []*SaveRequest, tx pgx.Tx) error {
	return nil
}

func (store *MockStore) Save(ctx context.Context, req *SaveRequest) error {
	return nil
}

func (store *MockStore) Delete(ctx context.Context, id int) error {
	return nil
}

func (store *MockStore) List(ctx context.Context, includeDeleted bool) ([]*Alert, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.listCount++
	return store.alerts, nil
}

func (store *MockStore) ListForSubscription(ctx context.Context, subName string) ([]*Alert, error) {
	return store.alerts, nil
}

func (store *MockStore) GetListCount() int {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return store.listCount
}

func TestCached(t *testing.T) {
	store := &MockStore{
		alerts: []*Alert{
			{
				IDAsString: "1",
			},
			{
				IDAsString: "3",
			},
		},
		mutex: sync.Mutex{},
	}
	provider, _ := NewConfigProvider(context.Background(), store, 10)
	alerts, err := provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.GetListCount())

	// Now call it again. This time it should not hit the store obj
	alerts, err = provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.GetListCount())
}

func TestCache_Refresh(t *testing.T) {
	store := &MockStore{
		alerts: []*Alert{
			{
				IDAsString: "1",
			},
			{
				IDAsString: "3",
			},
		},
		mutex: sync.Mutex{},
	}
	provider, _ := NewConfigProvider(context.Background(), store, 10)
	ctx := context.Background()
	alerts, err := provider.GetAllAlertConfigs(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.GetListCount())

	// Refresh should reset the cache
	_ = provider.Refresh(ctx)

	// Now call it again. This time it should hit the store obj
	alerts, err = provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 4, store.GetListCount())
}

func TestCache_Expire(t *testing.T) {
	store := &MockStore{
		alerts: []*Alert{
			{
				IDAsString: "1",
			},
			{
				IDAsString: "3",
			},
		},
		mutex: sync.Mutex{},
	}
	provider, _ := NewConfigProvider(context.Background(), store, 1)
	alerts, err := provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.GetListCount())

	// Sleep for 1 sec to expire the cache
	time.Sleep(1500 * time.Millisecond)

	// Now call it again. It should hit the store obj
	alerts, err = provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 4, store.GetListCount())
}

func TestCache_Concurrent(t *testing.T) {
	store := &MockStore{
		alerts: []*Alert{
			{
				IDAsString: "1",
			},
			{
				IDAsString: "3",
			},
		},
	}
	provider, _ := NewConfigProvider(context.Background(), store, 0)
	wg := sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			alerts, err := provider.GetAllAlertConfigs(context.Background(), false)
			require.NoError(t, err)
			assert.Equal(t, 2, len(alerts))
		}()
	}
	wg.Wait()
}
