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
	"skia.googlesource.com/buildbot.git/go/util"
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
	pool        *redis.Pool
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
		pool: &redis.Pool{
			MaxIdle:     1000,
			MaxActive:   0,
			IdleTimeout: time.Minute * 20,
			Dial: func() (redis.Conn, error) {
				c, err := redis.Dial("tcp", serverAddr)
				if err != nil {
					return nil, err
				}
				_, err = c.Do("SELECT", db)
				if err != nil {
					return nil, err
				}

				return c, err
			},
		},
	}
}

// Add, see the diff.LRUCache interface for details.
func (c *RedisLRUCache) Add(key, value interface{}) {
	prefixedKey, rawKey, err := c.encodeKey(key)
	if err != nil {
		glog.Errorf("Unable to create redis key: %s", err)
	}

	byteVal, err := c.encodeVal(value)
	if err != nil {
		glog.Errorf("Unable to create redis value: %s", err)
	}

	conn := c.pool.Get()
	defer conn.Close()

	conn.Send("MULTI")
	conn.Send("SET", prefixedKey, byteVal)
	conn.Send("SADD", c.indexSetKey, rawKey)
	_, err = conn.Do("EXEC")
	if err != nil {
		glog.Errorf("Unable to add key: %s", err)
	}
}

// Get, see the diff.LRUCache interface for details.
func (c *RedisLRUCache) Get(key interface{}) (interface{}, bool) {
	prefixedKey, _, err := c.encodeKey(key)
	if err != nil {
		glog.Errorf("Unable to create redis key %s", err)
	}

	conn := c.pool.Get()
	defer conn.Close()

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
	defer conn.Close()

	prefixedKey, rawKey, err := c.encodeKey(key)
	if err != nil {
		glog.Errorf("Unable to create redis key %s", err)
	}

	conn.Send("MULTI")
	conn.Send("DEL", prefixedKey)
	conn.Send("SREM", c.indexSetKey, rawKey)
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
	defer conn.Close()

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
	defer conn.Close()

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
