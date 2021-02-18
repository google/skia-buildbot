// Package reconnectingmemcached contains a wrapper around a general memcache client. It provides
// the ability to automatically reconnect after a certain number of failures. While the connection
// is down, its APIs quickly return, allowing clients to fallback to some other mechanism.
// This design decision (instead of, for example, blocking until the connection is restored) is
// because memcached is used where performance is critical, and it is probably faster for clients
// to respond to a memcached outage like they would a cache miss.
package reconnectingmemcached

import (
	"math/rand"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

type Client interface {
	// ConnectionAvailable returns true if there is an established connection. If false is returned,
	// it means the connection is being restored.
	ConnectionAvailable() bool
	// GetMulti returns a map filled with items that were in the cache. The boolean means "ok"
	// and can be false if either there was an error or the connection is currently down.
	GetMulti(keys []string) (map[string]*memcache.Item, bool)
	// Ping returns an error if there is no connection or if any instance is down.
	Ping() error
	// Set unconditionally sets the item. It returns false if there was an error or the connection
	// is currently down.
	Set(i *memcache.Item) bool
}

type Options struct {
	// Servers are the addresses of the servers that should be contacted with equal weight.
	// See bradfitz/gomemcache/memcache.New() for more.
	Servers []string
	// Timeout is the socket read/write timeout. The default is 100 milliseconds.
	Timeout time.Duration
	// MaxIdleConnections is the maximum number of connections. It should be greater than or
	// equal to the peek parallel requests. The default is 2.
	MaxIdleConnections int

	// AllowedFailuresBeforeHealing is the number of connection errors that will be tolerated
	// before autohealing starts.
	AllowedFailuresBeforeHealing int
}

type healingClientImpl struct {
	opts        Options
	client      *memcache.Client // if client is nil, that's a signal we are reconnecting.
	clientMutex sync.RWMutex
	numFailures int
}

func NewClient(opts Options) *healingClientImpl {
	c := memcache.New(opts.Servers...)
	c.Timeout = opts.Timeout                 // defaults handled from memcache client code.
	c.MaxIdleConns = opts.MaxIdleConnections // defaults handled from memcache client code.
	if opts.AllowedFailuresBeforeHealing <= 0 {
		opts.AllowedFailuresBeforeHealing = 10
	}
	return &healingClientImpl{
		opts:   opts,
		client: c,
	}
}

// ConnectionAvailable returns true if the clinet is not nil. nil means it is being healed.
func (h *healingClientImpl) ConnectionAvailable() bool {
	h.clientMutex.RLock()
	defer h.clientMutex.RUnlock()
	return h.client != nil
}

// GetMulti passes a call through to the underlying client (if available). If the connection
// is not available or there is an error, it returns false. Otherwise it returns the value and
// true.
func (h *healingClientImpl) GetMulti(keys []string) (map[string]*memcache.Item, bool) {
	h.clientMutex.RLock()
	if h.client == nil {
		// currently reconnecting
		h.clientMutex.RUnlock()
		return nil, false
	}
	m, err := h.client.GetMulti(keys)
	h.clientMutex.RUnlock() // need to free up the mutex before calling maybeReload
	if err != nil {
		sklog.Errorf("Could not get %d keys from memcached: %s", len(keys), err)
		h.maybeReload()
		return nil, false
	}
	return m, true
}

// Ping returns an error if the connection is being restored or any error from the
// underlying client.
func (h *healingClientImpl) Ping() error {
	h.clientMutex.RLock()
	defer h.clientMutex.RUnlock()
	if h.client == nil {
		return skerr.Fmt("Connection down. Reconnecting.")
	}
	return skerr.Wrap(h.client.Ping())
}

// Set passes through to the underlying client (if available). It returns true if the set succeeded
// or the passed in item is nil. It returns false if there was an error or the connection is down.
func (h *healingClientImpl) Set(i *memcache.Item) bool {
	if i == nil {
		return true // trivially true
	}
	h.clientMutex.RLock()
	if h.client == nil {
		// currently reconnecting
		h.clientMutex.RUnlock()
		return false
	}
	err := h.client.Set(i)
	h.clientMutex.RUnlock() // need to free up the mutex before calling maybeReload
	if err != nil {
		sklog.Errorf("Could not set item with key %s to memcached: %s", i.Key, err)
		h.maybeReload()
		return false
	}
	return true
}

// maybeReload will add one to the failure count. If that brings the number of failures over the
// limit, it will remove the connection and try to reconnect after 10-20 seconds.
func (h *healingClientImpl) maybeReload() {
	h.clientMutex.Lock()
	defer h.clientMutex.Unlock()
	h.numFailures++
	// We add the h.client == nil check to make it so there's only one goroutine in charge of
	// reconnecting
	if h.numFailures < h.numFailures || h.client == nil {
		return
	}
	sklog.Infof("Initiating memcached reconnection.")
	h.client = nil
	go func() { // spin up a background goroutine to heal the connection.
		for {
			// wait 10 seconds + some jitter to re-connect
			time.Sleep(10*time.Second + time.Duration(float32(10*time.Second)*rand.Float32()))
			c := memcache.New(h.opts.Servers...)
			c.Timeout = h.opts.Timeout
			c.MaxIdleConns = h.opts.MaxIdleConnections
			if err := c.Ping(); err != nil {
				sklog.Warningf("Cannot reconnect to memcached: %s", err)
				continue // go back to sleep, try again later
			}
			h.clientMutex.Lock()
			h.client = c
			h.numFailures = 0
			sklog.Infof("Reconnected to memcached")
			h.clientMutex.Unlock()
			return
		}
	}()
}

var _ Client = (*healingClientImpl)(nil)
