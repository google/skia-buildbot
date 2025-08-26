package alerts

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
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

	// Force refresh the cache because we know the source of truth has changed.
	Refresh(ctx context.Context) error
}

// ConfigCache struct contains cached alert config data.
type configCache struct {
	mutex                    sync.Mutex
	configs                  []*Alert
	expires                  time.Time
	includeDeleted           bool
	refreshIntervalInSeconds int
}

// ConfigProviderImpl implements ConfigProvider interface.
type configProviderImpl struct {
	alertStore  Store
	cacheActive *configCache
	cacheAll    *configCache
}

// newCache returns a new ConfigCache instance with the specified refresh interval.
func newCache(ctx context.Context, refreshIntervalInSeconds int, alertStore Store, now time.Time, includeDeleted bool) (*configCache, error) {
	ret := &configCache{
		configs:                  []*Alert{},
		expires:                  now.UTC().Add(time.Duration(refreshIntervalInSeconds * int(time.Second))),
		refreshIntervalInSeconds: refreshIntervalInSeconds,
		includeDeleted:           includeDeleted,
	}
	_, err := ret.getAlertConfigs(ctx, alertStore, now, true)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return ret, nil
}

func (c *configCache) getAlertConfigs(ctx context.Context, alertStore Store, now time.Time, force bool) ([]*Alert, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if now.After(c.expires) || force {
		alerts, err := alertStore.List(ctx, c.includeDeleted)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		c.configs = alerts
		c.expires = now.Add(time.Duration(c.refreshIntervalInSeconds * int(time.Second)))
	}
	return c.configs, nil
}

// NewConfigProvider returns a new instance of ConfigProvider interface.
func NewConfigProvider(ctx context.Context, alertStore Store, refreshIntervalInSeconds int) (ConfigProvider, error) {
	now := now.Now(ctx)
	cache_active, err := newCache(ctx, refreshIntervalInSeconds, alertStore, now, true)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	cache_all, err := newCache(ctx, refreshIntervalInSeconds, alertStore, now, false)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &configProviderImpl{
		alertStore:  alertStore,
		cacheActive: cache_active,
		cacheAll:    cache_all,
	}, nil
}

func (cp *configProviderImpl) Refresh(ctx context.Context) error {
	_, err := cp.cacheAll.getAlertConfigs(ctx, cp.alertStore, time.Now(), true)
	if err != nil {
		return skerr.Wrap(err)
	}
	_, err = cp.cacheActive.getAlertConfigs(ctx, cp.alertStore, time.Now(), true)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// GetAllAlertConfigs returns all alert configs.
func (cp *configProviderImpl) GetAllAlertConfigs(ctx context.Context, includeDeleted bool) ([]*Alert, error) {
	if includeDeleted {
		return cp.cacheAll.getAlertConfigs(ctx, cp.alertStore, time.Now(), false)
	} else {
		return cp.cacheActive.getAlertConfigs(ctx, cp.alertStore, time.Now(), false)
	}
}

var _ ConfigProvider = &configProviderImpl{}
