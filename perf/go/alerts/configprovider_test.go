package alerts

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockStore struct {
	alerts    []*Alert
	listCount int
}

func (store *MockStore) Save(ctx context.Context, cfg *Alert) error {
	return nil
}

func (store *MockStore) Delete(ctx context.Context, id int) error {
	return nil
}

func (store *MockStore) List(ctx context.Context, includeDeleted bool) ([]*Alert, error) {
	store.listCount++
	return store.alerts, nil
}

func (store *MockStore) GetListCount() int {
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
	}
	provider := NewConfigProvider(store, 10)
	alerts, err := provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 1, store.GetListCount())

	// Now call it again. This time it should not hit the store obj
	alerts, err = provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 1, store.GetListCount())
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
	}
	provider := NewConfigProvider(store, 10)
	alerts, err := provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 1, store.GetListCount())

	// Refresh should reset the cache
	provider.Refresh()

	// Now call it again. This time it should hit the store obj
	alerts, err = provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.GetListCount())
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
	}
	provider := NewConfigProvider(store, 1)
	alerts, err := provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 1, store.GetListCount())

	// Sleep for 1 sec to expire the cache
	time.Sleep(1 * time.Second)

	// Now call it again. It should hit the store obj
	alerts, err = provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.GetListCount())
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
	provider := NewConfigProvider(store, 0)
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
