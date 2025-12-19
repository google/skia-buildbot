/*
Package redis implements the redis related operations to support Skia perf, specifically
for the query UI. It includes two types of methods:
 1. The methods to interact with the Redis instances management on GCP.
    Those are done by using cloud.google.com/go/redis/apiv1
 2. The methods to interact with the Redis data on an Redis instance.
    Those are done by using github.com/redis/go-redis.
*/
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	gcp_redis "cloud.google.com/go/redis/apiv1"
	rpb "cloud.google.com/go/redis/apiv1/redispb"
	"github.com/redis/go-redis/v9"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/sklog"
)

// Format for the redis instance name in GCP.
const redisInstanceNameFormat = "projects/%s/locations/%s/instances/%s"

// RedisConfig contains properties of a redis instance.
type RedisConfig struct {
	// The GCP Project of the Redis instance
	Project string `json:"project,omitempty" optional:"true"`

	// The Zone (Region) of the Redis instance.
	Zone string `json:"zone,omitempty" optional:"true"`

	// The name of the Redis instance.
	Instance string `json:"instance,omitempty" optional:"true"`

	// Cache expiration for the given keys.
	CacheExpirationInMinutes int `json:"cache_expiration_minutes,omitempty" optional:"true"`
}

// redisCache implements RedisWrapper
type redisCache struct {
	gcpClient   *gcp_redis.CloudRedisClient
	config      *RedisConfig
	redisClient *redis.Client
}

// NewRedisCache returns an initialized RedisCache
func NewRedisCache(ctx context.Context, gcpClient *gcp_redis.CloudRedisClient, config *RedisConfig) (*redisCache, error) {
	r := &redisCache{
		gcpClient: gcpClient,
		config:    config,
	}
	err := r.init(ctx)
	return r, err
}

// init initializes the Redis cache connections.
func (r *redisCache) init(ctx context.Context) error {
	instanceName := fmt.Sprintf(redisInstanceNameFormat, r.config.Project, r.config.Zone, r.config.Instance)
	instanceRequest := &rpb.GetInstanceRequest{
		Name: instanceName,
	}
	instance, err := r.gcpClient.GetInstance(ctx, instanceRequest)
	if err != nil {
		sklog.Errorf("Error getting redis instance information %v", err)
		return err
	}
	opts := &redis.Options{
		Addr: fmt.Sprintf("%s:%d", instance.Host, instance.Port),
	}
	r.redisClient = redis.NewClient(opts)
	return nil
}

func (r *redisCache) Add(key string) {
	panic(errors.ErrUnsupported)
}

// Exists returns true  if the key is found in the cache.
func (r *redisCache) Exists(key string) bool {
	err := r.redisClient.Exists(context.Background(), key).Err()
	return err == nil
}

// SetValue sets the value for the key in the redis cache.
func (r *redisCache) SetValue(ctx context.Context, key string, value string) error {
	expirationMinutes := r.config.CacheExpirationInMinutes
	if expirationMinutes < 1 {
		// Default to 1 hour cache expiration.
		expirationMinutes = 60
	}
	expiryDuration := time.Minute * time.Duration(expirationMinutes)
	return r.redisClient.Set(ctx, key, value, expiryDuration).Err()
}

func (r *redisCache) SetValueWithExpiry(ctx context.Context, key string, value string, expiry time.Duration) error {
	return r.redisClient.Set(ctx, key, value, expiry).Err()
}

// GetValue returns the value for the key in the redis cache.
func (r *redisCache) GetValue(ctx context.Context, key string) (string, error) {
	value, err := r.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		sklog.Infof("Key %s not found", key)
		return "", nil
	}

	return value, err
}

// Confirm we implement the interface.
var _ cache.Cache = (*redisCache)(nil)
