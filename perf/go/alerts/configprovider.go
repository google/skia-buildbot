package alerts

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/skerr"
)

// ConfigProvider is an interface to retrieve alert configs.
type ConfigProvider interface {
	// GetAllAlertConfigs returns all alert configs.
	GetAllAlertConfigs(ctx context.Context, includeDeleted bool) ([]*Alert, error)

	// Refresh resets the configs and forces a fresh update.
	Refresh()
}

// ConfigCache struct contains cached alert config data.
type configCache struct {
	Configs                  []*Alert
	LastUpdated              time.Time
	refreshIntervalInSeconds int
}

// isExpired returns true if the data in the config cache is older.
// than the specified refresh interval.
func (cc *configCache) isExpired() bool {
	currentTime := time.Now().UTC()
	return len(cc.Configs) == 0 ||
		currentTime.Sub(cc.LastUpdated) > time.Duration(cc.refreshIntervalInSeconds)*time.Second
}

// ConfigProviderImpl implements ConfigProvider interface.
type configProviderImpl struct {
	alertStore               Store
	cache_active             *configCache
	cache_all                *configCache
	refreshIntervalInSeconds int
	mutex                    sync.Mutex
}

// newCache returns a new ConfigCache instance with the specified refresh interval.
func newCache(refreshIntervalInSeconds int) *configCache {
	return &configCache{
		Configs:                  []*Alert{},
		LastUpdated:              time.Now().UTC(),
		refreshIntervalInSeconds: refreshIntervalInSeconds,
	}
}

// NewConfigProvider returns a new instance of ConfigProvider interface.
func NewConfigProvider(alertStore Store, refreshIntervalInSeconds int) ConfigProvider {
	return &configProviderImpl{
		alertStore:               alertStore,
		cache_active:             newCache(refreshIntervalInSeconds),
		cache_all:                newCache(refreshIntervalInSeconds),
		refreshIntervalInSeconds: refreshIntervalInSeconds,
		mutex:                    sync.Mutex{},
	}
}

// GetAllAlertConfigs returns all alert configs
func (cp *configProviderImpl) GetAllAlertConfigs(ctx context.Context, includeDeleted bool) ([]*Alert, error) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()
	// Check if the relevant cache is expired. If so, reload it from the alert store.
	// There is still a chance of a race condition here where multiple threads find the
	// cache to have expired and will attempt to update it together. This should be an
	// acceptable tradeoff since adding a read lock above will prevent the thread from
	// getting a write lock and adding a lock for isExpired() might be overkill considering
	// the alert config data doesn't really change much.
	if (includeDeleted && cp.cache_all.isExpired()) ||
		(!includeDeleted && cp.cache_active.isExpired()) {
		err := cp.getAlertsFromStore(ctx, includeDeleted)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	if includeDeleted {
		return cp.cache_all.Configs, nil
	} else {
		return cp.cache_active.Configs, nil
	}
}

// Refresh resets the cache so that the next query forces a update from store.
func (cp *configProviderImpl) Refresh() {
	cp.cache_active = newCache(cp.refreshIntervalInSeconds)
	cp.cache_all = newCache(cp.refreshIntervalInSeconds)
}

// getAlertsFromStore returns the alert configs from the store.
func (cp *configProviderImpl) getAlertsFromStore(ctx context.Context, includeDeleted bool) error {
	alerts, err := cp.alertStore.List(ctx, includeDeleted)
	if err != nil {
		return skerr.Wrap(err)
	}

	if includeDeleted {
		cp.cache_all.Configs = alerts
		cp.cache_all.LastUpdated = time.Now().UTC()
	} else {
		cp.cache_active.Configs = alerts
		cp.cache_active.LastUpdated = time.Now().UTC()
	}

	return nil
}

var _ ConfigProvider = &configProviderImpl{}
