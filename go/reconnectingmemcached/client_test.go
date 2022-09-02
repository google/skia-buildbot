package reconnectingmemcached

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/stretchr/testify/assert"
)

func TestGetMulti_BooleanRepresentsConnectionState(t *testing.T) {

	hc, fmc := makeClientWithFakeMemcache()
	_, ok := hc.GetMulti([]string{"whatever"})
	assert.True(t, ok)

	fmc.isDown = true

	_, ok = hc.GetMulti([]string{"whatever"})
	assert.False(t, ok)
}

func TestSet_BooleanRepresentsConnectionState(t *testing.T) {

	hc, fmc := makeClientWithFakeMemcache()
	assert.True(t, hc.Set(&memcache.Item{}))

	fmc.isDown = true

	assert.False(t, hc.Set(&memcache.Item{}))
}

func TestPing_ReturnsErrorOnBadConnection(t *testing.T) {

	hc, fmc := makeClientWithFakeMemcache()
	assert.NoError(t, hc.Ping())

	fmc.isDown = true

	assert.Error(t, hc.Ping())
}

func TestRecovery_ConnectionReattemptedAfterAFewSeconds(t *testing.T) {

	hc, fmc := makeClientWithFakeMemcache()
	hc.numFailures = 5
	hc.recoveryDuration = time.Second
	fmc.isDown = true
	// Connection hasn't been detect as down yet
	assert.True(t, hc.ConnectionAvailable())

	// Inject a few more failures than required to make sure we don't block until healed.
	const failuresToInject = 10
	wc := sync.WaitGroup{}
	wc.Add(failuresToInject)
	for i := 0; i < failuresToInject; i++ {
		go func(isSet bool) {
			defer wc.Done()
			if isSet {
				assert.False(t, hc.Set(&memcache.Item{}))
			} else {
				_, ok := hc.GetMulti([]string{"whatever"})
				assert.False(t, ok)
			}
		}(i%2 == 0)
	}
	wc.Wait()
	// Connection should be down and healing
	assert.False(t, hc.ConnectionAvailable())
	// Things should be returning false
	assert.False(t, hc.Set(&memcache.Item{}))
	_, ok := hc.GetMulti([]string{"whatever"})
	assert.False(t, ok)
	assert.Error(t, hc.Ping())

	// Wait until we are sure the connection has been restored.
	time.Sleep(hc.recoveryDuration*2 + time.Second)

	// Connection should be back up
	assert.True(t, hc.ConnectionAvailable())
	// Things should be returning true again
	assert.True(t, hc.Set(&memcache.Item{}))
	_, ok = hc.GetMulti([]string{"whatever"})
	assert.True(t, ok)
	assert.NoError(t, hc.Ping())
}

func TestRecovery_HealsAfterThirdTry(t *testing.T) {

	const requiredRecoveryAttempts = 3
	recoveryAttempts := 0

	hc, fmc := makeClientWithFakeMemcache()
	hc.numFailures = 0
	hc.clientFactory = func(_ Options) memcachedClient {
		recoveryAttempts++
		if recoveryAttempts >= requiredRecoveryAttempts {
			fmc.recover()
		}
		return fmc
	}
	hc.recoveryDuration = time.Millisecond

	fmc.isDown = true

	_, ok := hc.GetMulti([]string{"whatever"})
	assert.False(t, ok)

	time.Sleep(time.Second)
	assert.True(t, hc.ConnectionAvailable())
	assert.Equal(t, 3, recoveryAttempts)

}

func makeClientWithFakeMemcache() (*healingClientImpl, *fakeMemcacheClient) {
	fmc := &fakeMemcacheClient{}
	return &healingClientImpl{
		client: fmc,
		clientFactory: func(_ Options) memcachedClient {
			// Call recover to signal connection restored and then return the
			// same client to make it easy to handle assertions.
			fmc.recover()
			return fmc
		},
	}, fmc
}

type fakeMemcacheClient struct {
	isDown bool
	mutex  sync.RWMutex
}

func (f *fakeMemcacheClient) Ping() error {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	if f.isDown {
		return errors.New("down")
	}
	return nil
}

func (f *fakeMemcacheClient) GetMulti(_ []string) (map[string]*memcache.Item, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	if f.isDown {
		return nil, errors.New("down")
	}
	return map[string]*memcache.Item{}, nil
}

func (f *fakeMemcacheClient) Set(_ *memcache.Item) error {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	if f.isDown {
		return errors.New("down")
	}
	return nil
}

func (f *fakeMemcacheClient) recover() {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.isDown = false
}
