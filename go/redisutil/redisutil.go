// Package redisutil provides helper types that use Redis as a backend stored
// instead of RAM.
// Such caches are available after restarts and can be shared among
// multiple machines.
package redisutil

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
)

const (
	// Constants that identify that value type for serialization/deserialization.
	BYTES_TYPE  = "b"
	STRING_TYPE = "s"
	INT_TYPE    = "i"
	JSON_TYPE   = "j"
	CODEC_TYPE  = "c"
)

// RedisLRUCache is a Redis backed LRU cache.
type RedisLRUCache struct {
	indexSetKey string
	keyPrefix   string
	codec       util.LRUCodec
	pool        *RedisPool
}

// NewRedisLRUCache returns a new Redis backed cache that complies with the
// util.LRUCache interface.
// TODO(stephana): This is a misnomer since NewRedisLRUCache does not
// expunge items automatically based on a timestamp. This needs to be fixed.
func NewRedisLRUCache(serverAddr string, db int, id string, codec util.LRUCodec) util.LRUCache {
	return &RedisLRUCache{
		keyPrefix:   id + ":",
		indexSetKey: id + ":idx",
		codec:       codec,
		pool:        NewRedisPool(serverAddr, db),
	}
}

// Add, see the diff.LRUCache interface for details.
func (c *RedisLRUCache) Add(key, value interface{}) bool {
	prefixedKey, rawKey, err := c.encodeKey(key)
	if err != nil {
		glog.Errorf("Unable to create redis key: %s", err)
		return false
	}

	byteVal, err := c.encodeVal(value)
	if err != nil {
		glog.Errorf("Unable to create redis value: %s", err)
		return false
	}

	conn := c.pool.Get()
	defer util.Close(conn)

	util.LogErr(conn.Send("MULTI"))
	util.LogErr(conn.Send("SET", prefixedKey, byteVal))
	util.LogErr(conn.Send("SADD", c.indexSetKey, rawKey))
	_, err = conn.Do("EXEC")
	if err != nil {
		glog.Errorf("Unable to add key: %s", err)
		return false
	}
	return true
}

// Get, see the diff.LRUCache interface for details.
func (c *RedisLRUCache) Get(key interface{}) (interface{}, bool) {
	prefixedKey, _, err := c.encodeKey(key)
	if err != nil {
		glog.Errorf("Unable to create redis key %s", err)
	}

	conn := c.pool.Get()
	defer util.Close(conn)

	ret, err := c.decodeVal(redis.Bytes(conn.Do("GET", prefixedKey)))
	if err != nil {
		// Only log an error if it's not a missing value.
		if err != redis.ErrNil {
			glog.Errorf("Unable to get key %s: %s", prefixedKey, err)
		}
		return nil, false
	}
	return ret, true
}

// Remove, see the diff.LRUCache interface for details.
func (c *RedisLRUCache) Remove(key interface{}) {
	conn := c.pool.Get()
	defer util.Close(conn)

	prefixedKey, rawKey, err := c.encodeKey(key)
	if err != nil {
		glog.Errorf("Unable to create redis key %s", err)
	}

	util.LogErr(conn.Send("MULTI"))
	util.LogErr(conn.Send("DEL", prefixedKey))
	util.LogErr(conn.Send("SREM", c.indexSetKey, rawKey))
	_, err = conn.Do("EXEC")
	if err != nil {
		glog.Errorf("Error deleting key:%s", err)
	}
}

// Purge clears the cache.
func (c *RedisLRUCache) Purge() {
	for _, k := range c.Keys() {
		c.Remove(k)
	}
}

// Keys returns all current keys in the cache.
func (c *RedisLRUCache) Keys() []interface{} {
	conn := c.pool.Get()
	defer util.Close(conn)

	ret, err := redis.Values(conn.Do("SMEMBERS", c.indexSetKey))
	if err != nil {
		glog.Errorf("Unable to get keys: %s", err)
		return nil
	}

	result := make([]interface{}, len(ret))
	for i, v := range ret {
		temp, ok := v.([]byte)
		if !ok {
			glog.Errorf("Unable to decode key: %v", v)
			return nil
		}
		result[i] = c.decodeKey(temp)
	}
	return result
}

// Len, see the diff.LRUCache interface for details.
func (c *RedisLRUCache) Len() int {
	conn := c.pool.Get()
	defer util.Close(conn)

	ret, err := redis.Int(conn.Do("SCARD", c.indexSetKey))
	if err != nil {
		glog.Errorf("Unable to get length: %s", err)
		return 0
	}
	return ret
}

func (c *RedisLRUCache) encodeVal(val interface{}) ([]byte, error) {
	var resultVal []byte

	switch testVal := val.(type) {
	// Use []byte directly.
	case []byte:
		resultVal = []byte(BYTES_TYPE + string(testVal))
	// Cast strings to []byte.
	case string:
		resultVal = []byte(STRING_TYPE + testVal)
	case int:
		resultVal = []byte(INT_TYPE + strconv.Itoa(testVal))
	default:
		// If we have a codec then decode it.
		if c.codec == nil {
			return nil, fmt.Errorf("Values cannot be of type: %v", reflect.TypeOf(val))
		}
		bytesVal, err := c.codec.Encode(val)
		if err != nil {
			return nil, fmt.Errorf("Unable to encode %v. Got error: %s", val, err)
		}
		resultVal = []byte(CODEC_TYPE + string(bytesVal))
	}
	return resultVal, nil
}

func (c *RedisLRUCache) decodeVal(val []byte, err error) (interface{}, error) {
	if err != nil {
		return nil, err
	}

	switch string(val[:1]) {
	case BYTES_TYPE:
		return val[1:], nil
	case STRING_TYPE:
		return string(val[1:]), nil
	case INT_TYPE:
		ret, err := strconv.ParseInt(string(val[1:]), 10, 0)
		return int(ret), err
	case CODEC_TYPE:
		if c.codec == nil {
			return nil, errors.New("No codec defined to decode byte array")
		}
		return c.codec.Decode(val[1:])
	default:
		return nil, fmt.Errorf("Unable to decode value: %s", string(val))
	}
}

func (c *RedisLRUCache) encodeKey(key interface{}) (string, string, error) {
	keyStr, ok := key.(string)
	if !ok {
		return "", "", errors.New("Key values have to be of type 'strings'")
	}
	return c.keyPrefix + keyStr, keyStr, nil
}

func (c *RedisLRUCache) decodeKey(key []byte) interface{} {
	return string(key)
}

// RedisPool embeds an instance of redis.Pool and provides utility functions
// to deal with common tasks when talking to Redis.
type RedisPool struct {
	redis.Pool
	db int
}

// NewRedisPool returns a new instance of RedisPool that connects to the given
// server and database.
func NewRedisPool(serverAddr string, db int) *RedisPool {
	dial := func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", serverAddr)
		if err != nil {
			return nil, err
		}
		_, err = c.Do("SELECT", db)
		if err != nil {
			return nil, err
		}

		return c, err
	}

	return &RedisPool{
		Pool: redis.Pool{
			MaxIdle:     1000,
			MaxActive:   1000,
			Wait:        true,
			IdleTimeout: time.Minute * 20,
			Dial:        dial,
		},
		db: db,
	}
}

// FlushDB clears the database that is used by this RedisPool instance.
func (r *RedisPool) FlushDB() error {
	c := r.Get()
	defer util.Close(c)

	_, err := c.Do("FLUSHDB")
	return err
}

// subscribeToChannel is a utility function that converts a Redis PubSub channel
// into a Go channel. Upon sucessful subscription it will send an empty string
// to the channel. This is also done if it needs to reconnect to the channel.
// This call will block until either an error has occured or the subcription
// was succesful.
func (r *RedisPool) subscribeToChannel(channel string) (<-chan []byte, error) {
	psc := redis.PubSubConn{Conn: r.Get()}
	if err := psc.Subscribe(channel); err != nil {
		return nil, err
	}

	ret := make(chan []byte)
	readyCh := make(chan bool)
	go func() {
		for {
		Loop:
			for {
				switch v := psc.Receive().(type) {
				case redis.Message:
					ret <- v.Data
				case redis.Subscription:
					readyCh <- true
					ret <- []byte("")
				case error:
					glog.Errorf("Error while waiting for PUBSUB messages. ")
					break Loop

				}
			}
			util.Close(psc)
			// Keep trying to reconnect. An error here should be the exception.
			for {
				psc := redis.PubSubConn{Conn: r.Get()}
				if err := psc.Subscribe(channel); err != nil {
					glog.Errorf("Error connecting to pubsub channel: %s", channel)
					// This should be the rare exception and will mostly mean that
					// Redis or the network connection is down.
					time.Sleep(5 * time.Second)
				} else {
					break
				}
			}
		}
	}()
	<-readyCh

	return ret, nil
}

// List returns a channel that sends the elements of a Redis list
// as they are arrive. This is the complement to the AddToList function.
func (r *RedisPool) List(listKey string) <-chan []byte {
	ret := make(chan []byte)

	go func() {
		for {
			c := r.Get()
			for {
				reply, err := redis.Values(c.Do("BLPOP", listKey, 0))
				if err != nil {
					glog.Errorf("Error retrieving list. Reconnecting.: %s", err)
					break
				}
				// The returned err cannot be different from nil. We are passing nil
				// as the err argument and any other error is already captured above.
				data, _ := redis.Bytes(reply[1], nil)
				ret <- data
			}
			util.Close(c)
		}
	}()

	return ret
}

// AppendList adds to the end of a Redis list.
func (r *RedisPool) AppendList(listKey string, data []byte) error {
	c := r.Get()
	defer util.Close(c)

	_, err := c.Do("RPUSH", listKey, data)
	return err
}

// SaveHash saves 'data' in a Redis hash. It's assumed that data is an instance
// of a struct.
func (r *RedisPool) SaveHash(hashKey string, data interface{}) error {
	c := r.Get()
	defer util.Close(c)

	_, err := c.Do("HMSET", redis.Args{}.Add(hashKey).AddFlat(data)...)
	return err
}

// LoadHashToStruct loads a Redis hash into the provided target structure.
func (r *RedisPool) LoadHashToStruct(hashKey string, targetStruct interface{}) (bool, error) {
	c := r.Get()
	defer util.Close(c)

	vals, err := redis.Values(c.Do("HGETALL", hashKey))
	if err != nil {
		return false, err
	}

	if len(vals) == 0 {
		return false, nil
	}

	if err := redis.ScanStruct(vals, targetStruct); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteKey deletes the given key from the Redis database.
func (r *RedisPool) DeleteKey(key string) error {
	c := r.Get()
	defer util.Close(c)

	_, err := c.Do("DEL", key)
	return err
}
