package alerts

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/now"
)

type MockStore struct {
	alerts    []*Alert
	listCount int
	mutex     sync.Mutex
}

func (store *MockStore) Save(ctx context.Context, cfg *Alert) error {
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

func (store *MockStore) numberOfTimesListCalled() int {
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
	provider, err := NewConfigProvider(context.Background(), store, 10)
	require.NoError(t, err)

	alerts, err := provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.numberOfTimesListCalled())

	// Now call it again. This time it should not hit the store obj
	alerts, err = provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.numberOfTimesListCalled())
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
	provider, err := NewConfigProvider(context.Background(), store, 10)
	require.NoError(t, err)
	ctx := context.Background()
	alerts, err := provider.GetAllAlertConfigs(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.numberOfTimesListCalled())

	// Now call it again. This time it should not hit the store obj
	alerts, err = provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.numberOfTimesListCalled())
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
	provider, err := NewConfigProvider(context.Background(), store, 1)
	require.NoError(t, err)
	alerts, err := provider.GetAllAlertConfigs(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 2, store.numberOfTimesListCalled())

	// Sleep for 2 sec to expire the cache
	time.Sleep(3 * time.Second)

	// Use TimeTravelingContext here.
	futureCtx := now.TimeTravelingContext(time.Now().Add(time.Duration(3 * time.Second)))

	// Now call it again. It should hit the store just once.
	alerts, err = provider.GetAllAlertConfigs(futureCtx, false)
	require.NoError(t, err)
	assert.Equal(t, 2, len(alerts))
	assert.Equal(t, 3, store.numberOfTimesListCalled())
}
