package redisutil

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/rtcache"
	"go.skia.org/infra/go/util"
)

const (
	// QUEUE_PREFIX is the prefix for the task queue which is a sorted set in Redis.
	QUEUE_PREFIX = "rc:"
)

// RedisRTC implements a rtcache.ReadThroughCache with a Redis backend.
type RedisRTC struct {
	// redisPool is the connection to redis this read-through-cache uses.
	redisPool *RedisPool

	// queueKey is the key of the work queue (sorted set) in Redis.
	queueKey string

	// inProgressKey is the key of the set of current tasks in progress.
	inProgressKey string

	// keyPrefix is the prefix of the results that are cached in redis.
	keyPrefix string

	// errKeyPrefix is the prefix of the error message if the task failed to
	// produce the desired resul.
	errKeyPrefix string

	// queueSubChannel is the channel where keyspace notifications are generated
	// when a new item is added to the task queue.
	queueSubChannel string

	// finishedChannel is the key of the PubSub channel where finished tasked are
	// announced.
	finishedChannel string

	// codec is used to serialize and desirialize the cached items.
	codec util.LRUCodec

	// worker is the function that is called to produce an item when it is not
	// cached.
	worker rtcache.ReadThroughFunc

	// doneCh is a channel to subscribe to be notified when a task is done.
	doneCh chan<- *doneSub
}

// workerTask is an auxiliary struct that contains the ID of a task
// and it's priority.
type workerTask struct {
	id       string
	priority int64
}

// doneSub is an auxiliary struct that is used by tasks to subscribe to be
// notified when a requested item is available because it has been generated
// by calling the worker. It is sent via the 'done' channel in RedisRTC.
type doneSub struct {
	id       string
	notifyCh chan bool
}

// Returns a new Redis based ReadThroughCache. 'queuename' has to be unqiue
// within the given RedisPool. The 'worker' function is called if the
// requested item is not the cache. 'nWorkers' specifies how many concurrent
// workers should be started.
func NewReadThroughCache(redisPool *RedisPool, queueName string, worker rtcache.ReadThroughFunc, codec util.LRUCodec, nWorkers int) (rtcache.ReadThroughCache, error) {
	c := redisPool.Get()
	defer util.Close(c)

	// Make sure keyspace notifications are enabled.
	if _, err := c.Do("CONFIG", "SET", "notify-keyspace-events", "AKE"); err != nil {
		return nil, err
	}

	queueKey := QUEUE_PREFIX + queueName
	keyPrefix := queueKey + ":k:"
	errKeyPrefix := queueKey + ":er:"
	finishedChannel := queueKey + ":ch"

	ret := &RedisRTC{
		redisPool:       redisPool,
		queueKey:        queueKey,
		inProgressKey:   queueKey + ":inp",
		finishedChannel: finishedChannel,
		keyPrefix:       keyPrefix,
		errKeyPrefix:    errKeyPrefix,
		queueSubChannel: fmt.Sprintf("__keyspace@%d__:%s", redisPool.db, queueKey),
		codec:           codec,
		worker:          worker,
	}

	// Start the feeder process.
	var err error
	if ret.doneCh, err = ret.startWorkScheduler(); err != nil {
		return nil, err
	}

	// Start the workers if we have a worker specified.
	if worker != nil {
		if err = ret.startWorkers(nWorkers); err != nil {
			return nil, err
		}
	}

	// Start the background process that runs the workers.
	return ret, nil
}

// Get implements the rtcache.ReadThroughCache interface. See details there.
func (r *RedisRTC) Get(priority int64, returnBytes bool, id string) (interface{}, error) {
	// Look it up in Redis.
	ret, err := r.getResultErr(id, returnBytes)
	if (ret != nil) || (err != nil) {
		return ret, err
	}

	// Else queue in the request.
	return r.waitFor(id, priority, returnBytes)
}

func (r *RedisRTC) startWorkers(nWorkers int) error {
	workCh, err := r.getWorkChannel()
	if err != nil {
		return err
	}

	for i := 0; i < nWorkers; i++ {
		go func() {
			for {
				task := <-workCh
				r.writeWorkerResult(task.priority, task.id)
			}
		}()
	}

	return nil
}

// getWorkChannel returns a channel that sends tasks to be processed.
func (r *RedisRTC) getWorkChannel() (<-chan *workerTask, error) {
	ret := make(chan *workerTask)

	subCh, err := r.getKeyEventChannel(r.queueSubChannel, false)
	if err != nil {
		return nil, err
	}

	go func() {
		hasMore := true
		for {
			if !hasMore {
				<-subCh
			}

			// Get the next task.
			workTask, itemsLeft, err := r.dequeue()
			if err != nil {
				glog.Errorf("Error moving work ids: %s", err)
				continue
			}

			hasMore = itemsLeft > 0
			if workTask != nil {
				ret <- workTask
			}
		}
	}()

	return ret, nil
}

// dequeue returns a task to be performed by the worker.
// It returns a triple: workerTask, itemsLeft_in_the_task_queue, error.
func (r *RedisRTC) dequeue() (*workerTask, int, error) {
	c := r.redisPool.Get()
	defer util.Close(c)

	// TODO: this needs to be a transaction.
	util.LogErr(c.Send("MULTI"))
	util.LogErr(c.Send("ZRANGE", r.queueKey, 0, 0, "WITHSCORES"))
	util.LogErr(c.Send("ZREMRANGEBYRANK", r.queueKey, 0, 0))
	util.LogErr(c.Send("ZCARD", r.queueKey))
	combinedVals, err := redis.Values(c.Do("EXEC"))

	if err != nil {
		return nil, 0, err
	}

	// Get the number of found elements.
	count := combinedVals[1].(int64)

	// If there are no values, we are done.
	if count == 0 {
		return nil, 0, nil
	}

	result := combinedVals[0].([]interface{})
	id := string(result[0].([]byte))
	priority, err := redis.Int64(result[1], nil)
	if err != nil {
		return nil, 0, err
	}
	itemsLeft := int(combinedVals[2].(int64))
	ret := &workerTask{id, priority}

	args := append([]interface{}{r.inProgressKey}, id)
	if _, err := c.Do("SADD", args...); err != nil {
		return nil, 0, err
	}

	return ret, itemsLeft, nil
}

// writeWorkerResult runs the worker and writes the result to Redis.
func (r *RedisRTC) writeWorkerResult(priority int64, id string) {
	result, err := r.worker(priority, id)
	var writeKey string
	var writeData []byte
	if err != nil {
		writeKey = r.errorKey(id)
		writeData = []byte(err.Error())
	} else {
		if writeData, err = r.codec.Encode(result); err != nil {
			writeKey = r.errorKey(id)
			writeData = []byte(fmt.Sprintf("Error encoding worker result: %s", err))
		} else {
			writeKey = r.key(id)
		}
	}

	c := r.redisPool.Get()
	defer util.Close(c)

	util.LogErr(c.Send("MULTI"))
	util.LogErr(c.Send("SET", writeKey, writeData))
	util.LogErr(c.Send("SREM", r.inProgressKey, id))

	// Expire the error after 10 seconds to let the client decide
	// whether we need to retry.
	if err != nil {
		util.LogErr(c.Send("EXPIRE", writeKey, 10))
	}
	util.LogErr(c.Send("PUBLISH", r.finishedChannel, id))
	if _, err = c.Do("EXEC"); err != nil {
		glog.Errorf("Error writing result to redis: %s", err)
	}
}

// getKeyEventChannel returns a channel that sends events for key changes in
// Redis.
func (r *RedisRTC) getKeyEventChannel(channelOrPattern string, isPattern bool) (<-chan string, error) {
	// Listen for changes on the queue continously.
	psc := redis.PubSubConn{Conn: r.redisPool.Get()}

	subscribe := func() error {
		if isPattern {
			return psc.PSubscribe(channelOrPattern)
		} else {
			return psc.Subscribe(channelOrPattern)
		}
	}

	// Subscribe to the key events
	if err := subscribe(); err != nil {
		return nil, err
	}

	readyCh := make(chan bool)
	ret := make(chan string)
	go func() {
		for {
		Loop:
			for {
				switch v := psc.Receive().(type) {
				case redis.PMessage:
					ret <- string(v.Data)
				case redis.Message:
					ret <- string(v.Data)
				case redis.Subscription:
					if readyCh != nil {
						readyCh <- true
						close(readyCh)
					}
				case error:
					glog.Errorf("Error waiting for key events: %s", v)
					glog.Infof("Reconnecting.")
					util.Close(psc)
					break Loop
				}
			}

			readyCh = nil
			psc = redis.PubSubConn{Conn: r.redisPool.Get()}
			if err := subscribe(); err != nil {
				glog.Errorf("Error re-connecting: %s", err)
				time.Sleep(time.Second)
			}
		}
	}()
	<-readyCh

	return ret, nil
}

// enqueue adds the given task and priority to the task queue. Updating the
// priority if necessary.
func (r *RedisRTC) enqueue(id string, priority int64) (bool, error) {
	c := r.redisPool.Get()
	defer util.Close(c)

	util.LogErr(c.Send("MULTI"))
	util.LogErr(c.Send("ZSCORE", r.queueKey, id))
	util.LogErr(c.Send("SISMEMBER", r.inProgressKey, id))
	util.LogErr(c.Send("EXISTS", r.key(id)))
	util.LogErr(c.Send("EXISTS", r.errorKey(id)))
	retVals, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return false, err
	}

	// See if the id is in the queue or in progress.
	inQueueScore, isInProgress := retVals[0], retVals[1].(int64) == 1
	foundResult, foundError := retVals[2].(int64), retVals[3].(int64)
	found := (foundResult + foundError) > 0

	// If the calculation is in process we don't have todo anything.
	if !isInProgress && !found {
		saveId := true

		// Only update the queue if this has a lower score.
		if inQueueScore != nil {
			oldPriority, _ := redis.Int64(inQueueScore, nil)
			saveId = priority < oldPriority
		}

		if saveId {
			if _, err = c.Do("ZADD", r.queueKey, priority, id); err != nil {
				return false, err
			}
		}
	}

	return found, nil
}

// inQueue returns up to 'maxElements' ids that are currently in the work
// queue. This is primarily for testing.
func (r *RedisRTC) inQueue(maxElements int) ([]string, error) {
	c := r.redisPool.Get()
	defer util.Close(c)

	return redis.Strings(c.Do("ZRANGE", r.queueKey, 0, maxElements-1))
}

// inProgress returns the items that are currently in progress of being
// calculated. This is primarily for testing.
func (r *RedisRTC) inProgress() ([]string, error) {
	c := r.redisPool.Get()
	defer util.Close(c)

	return redis.Strings(c.Do("SMEMBERS", r.inProgressKey))
}

// isFinished returns true if the task has finished.
func (r *RedisRTC) isFinished(id string) bool {
	c := r.redisPool.Get()
	defer util.Close(c)

	util.LogErr(c.Send("MULTI"))
	util.LogErr(c.Send("EXISTS", r.key(id)))
	util.LogErr(c.Send("EXISTS", r.errorKey(id)))
	resArr, err := redis.Ints(c.Do("EXEC"))
	if err != nil {
		glog.Errorf("Unable to check if key exits: %s", err)
		return false
	}

	return (resArr[0] + resArr[1]) > 0
}

// startWorkScheduler starts the background progress that takes tasks
// from the doneCh and adds them to the Redis priority queue.
func (r *RedisRTC) startWorkScheduler() (chan<- *doneSub, error) {
	finishedCh, err := r.redisPool.subscribeToChannel(r.finishedChannel)
	if err != nil {
		return nil, err
	}

	doneCh := make(chan *doneSub)
	go func() {
		watchIds := map[string][]chan bool{}
		notifyChannels := func(id string) {
			for _, ch := range watchIds[id] {
				ch <- true
				close(ch)
			}
			delete(watchIds, id)
		}

		notifyAll := func() {
			for id := range watchIds {
				if r.isFinished(id) {
					notifyChannels(id)
				}
			}
		}

		for {
			select {
			case subscription := <-doneCh:
				watchIds[subscription.id] = append(watchIds[subscription.id], subscription.notifyCh)
				if r.isFinished(subscription.id) {
					notifyChannels(subscription.id)
				}
			case finishedId := <-finishedCh:
				// An emtpy string indicates that we (re)connected.
				if string(finishedId) == "" {
					notifyAll()
				} else {
					notifyChannels(string(finishedId))
				}
			}
		}
	}()

	return doneCh, nil
}

// waitFor blocks until the key identified by id is available in Redis.
func (r *RedisRTC) waitFor(id string, priority int64, returnBytes bool) (interface{}, error) {
	var found bool
	var err error
	if found, err = r.enqueue(id, priority); err != nil {
		return nil, err
	}

	if !found {
		finishedCh := make(chan bool, 1)
		r.doneCh <- &doneSub{id, finishedCh}
		<-finishedCh
	}

	ret, err := r.getResultErr(id, returnBytes)
	if err != nil {
		return nil, err
	}

	if ret == nil {
		return nil, fmt.Errorf("Unable to retrieve result for id: %s", id)
	}

	return ret, nil
}

// getResultErr returns the cached version of the item identified by 'id' or
// nil if it's not available.
func (r *RedisRTC) getResultErr(id string, returnBytes bool) (interface{}, error) {
	c := r.redisPool.Get()
	defer util.Close(c)

	util.LogErr(c.Send("MULTI"))
	util.LogErr(c.Send("GET", r.key(id)))
	util.LogErr(c.Send("GET", r.errorKey(id)))
	resArr, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return nil, err
	}
	retBytes, errBytes := resArr[0], resArr[1]
	if errBytes != nil {
		return nil, fmt.Errorf("For %s we received error: %s", id, string(errBytes.([]byte)))
	}

	if retBytes != nil {
		if returnBytes {
			return retBytes, nil
		}
		return r.codec.Decode(retBytes.([]byte))
	}

	// We have neither an error nor any data.
	return nil, nil
}

// key returns the Redis key for the given id.
func (r *RedisRTC) key(id string) string {
	return r.keyPrefix + id
}

// errorKey returns the key of the error message for the given ID if the
// the worker call failed.
func (r *RedisRTC) errorKey(id string) string {
	return r.errKeyPrefix + id
}
