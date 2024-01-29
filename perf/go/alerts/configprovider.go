package alerts

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

type WhatToInclude bool

const (
	ReadAlertsTimeout   time.Duration = time.Minute
	IncludeDeleted      WhatToInclude = true
	DoNotIncludeDeleted WhatToInclude = false
)

// ConfigProvider is an interface to retrieve alert configs.
type ConfigProvider interface {
	// GetAllAlertConfigs returns all alert configs.
	GetAllAlertConfigs(ctx context.Context, includeDeleted bool) ([]*Alert, error)

	// Refresh resets the configs and forces a fresh update.
	Refresh(ctx context.Context) error
}

// ConfigCache struct contains cached alert config data.
type configCache struct {
	Configs                  []*Alert
	LastUpdated              time.Time
	refreshIntervalInSeconds int
}

// ConfigProviderImpl implements ConfigProvider interface.
type configProviderImpl struct {
	alertStore               Store
	cache_active             *configCache
	cache_all                *configCache
	refreshIntervalInSeconds int
	mutex                    sync.RWMutex
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
func NewConfigProvider(ctx context.Context, alertStore Store, refreshIntervalInSeconds int) (ConfigProvider, error) {
	provider := &configProviderImpl{
		alertStore:               alertStore,
		cache_active:             newCache(refreshIntervalInSeconds),
		cache_all:                newCache(refreshIntervalInSeconds),
		refreshIntervalInSeconds: refreshIntervalInSeconds,
		mutex:                    sync.RWMutex{},
	}

	err := provider.startRefresher(ctx)
	return provider, err
}

// GetAllAlertConfigs returns all alert configs.
func (cp *configProviderImpl) GetAllAlertConfigs(ctx context.Context, includeDeleted bool) ([]*Alert, error) {
	cp.mutex.RLock()
	defer cp.mutex.RUnlock()

	if includeDeleted {
		return cp.cache_all.Configs, nil
	} else {
		return cp.cache_active.Configs, nil
	}
}

// Refresh resets the cache by updating data from store.
func (cp *configProviderImpl) Refresh(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, ReadAlertsTimeout)
	defer cancel()

	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Update the "all" configs.
	err := cp.getAlertsFromStore(timeoutCtx, IncludeDeleted)
	if err != nil {
		// Log the error, but let it continue to give the active config update a chance.
		sklog.Errorf("Error when retrieving ALL alert configs %s.", err)
	}

	// Update the "active" configs.
	err = cp.getAlertsFromStore(timeoutCtx, DoNotIncludeDeleted)
	if err != nil {
		return err
	}

	return nil
}

func (cp *configProviderImpl) startRefresher(ctx context.Context) error {
	// Refresh it once to fill the cache upon initiation.
	err := cp.Refresh(ctx)
	if err != nil {
		return err
	}

	// Update the cache periodically.
	go func() {
		// Periodically run it based on the specified duration.
		refreshDuration := time.Second * time.Duration(cp.refreshIntervalInSeconds)
		for range time.Tick(refreshDuration) {
			err := cp.Refresh(ctx)
			if err != nil {
				sklog.Errorf("Error updating alert configurations. %s", err)
			}
		}
	}()

	return nil
}

// getAlertsFromStore retrieves the alert configs from the store and updates the cache.
func (cp *configProviderImpl) getAlertsFromStore(ctx context.Context, whatToInclude WhatToInclude) error {
	alerts, err := cp.alertStore.List(ctx, bool(whatToInclude))
	if err != nil {
		return skerr.Wrap(err)
	}

	if whatToInclude == IncludeDeleted {
		cp.cache_all.Configs = alerts
		cp.cache_all.LastUpdated = time.Now().UTC()
	} else {
		cp.cache_active.Configs = alerts
		cp.cache_active.LastUpdated = time.Now().UTC()
	}

	return nil
}

var _ ConfigProvider = &configProviderImpl{}
