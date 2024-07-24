/*
Package redis implements the redis related operations to support Skia perf, specifically
for the query UI. It includes two types of methods:
 1. The methods to interact with the Redis instances management on GCP.
    Those are done by using cloud.google.com/go/redis/apiv1
 2. TODO(wenbinzhang) The methods to interact with the Redis data on an Redis instance.
    Those are done by using github.com/redis/go-redis.
*/
package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	gcp_redis "cloud.google.com/go/redis/apiv1"
	rpb "cloud.google.com/go/redis/apiv1/redispb"
	"github.com/redis/go-redis/v9"
	redis_client "go.skia.org/infra/go/cache/redis"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore"
	"google.golang.org/api/iterator"
)

// RedisWrapper define the interfaces for Redis related operations
type RedisWrapper interface {
	StartRefreshRoutine(context.Context, time.Duration, *config.InstanceConfig) error
	ListRedisInstances(context.Context, string) []*rpb.Instance
}

// RedisClient implements RedisWrapper
type RedisClient struct {
	gcpClient    *gcp_redis.CloudRedisClient
	traceStore   *tracestore.TraceStore
	tilesToCache int

	mutex sync.Mutex
}

// Create a new Redisclient.
func NewRedisClient(ctx context.Context, gcpClient *gcp_redis.CloudRedisClient, traceStore *tracestore.TraceStore, tilesToCache int) *RedisClient {
	return &RedisClient{
		gcpClient:    gcpClient,
		traceStore:   traceStore,
		tilesToCache: tilesToCache,
	}
}

// Start a routine to periodically access the Redis instance.
func (r *RedisClient) StartRefreshRoutine(ctx context.Context, refreshPeriod time.Duration, config *redis_client.RedisConfig) {
	project := config.Project
	zone := config.Zone
	if project == "" || zone == "" {
		sklog.Errorf("No project or zone defined in redis config.")
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", project, zone)
	sklog.Infof("Start listing Redis instances for %s.", parent)
	go func() {
		for range time.Tick(refreshPeriod) {
			sklog.Infof("Time to list Redis instances...")
			var sb strings.Builder
			instances := r.ListRedisInstances(ctx, parent)
			var targetInstance *rpb.Instance
			sb.WriteString(fmt.Sprintf("Found %d Redis instances.\n", len(instances)))
			for _, instance := range instances {
				sb.WriteString(fmt.Sprintf("Name: %s, Host: %s, Port: %d\n", instance.Name, instance.Host, instance.Port))
				namePieces := strings.Split(instance.Name, "/")
				name := namePieces[len(namePieces)-1]
				if name == config.Instance {
					sb.WriteString(fmt.Sprintf("Target instance found: %s", config.Instance))
					targetInstance = instance
				}
			}
			sklog.Infof(sb.String())
			if targetInstance != nil {
				r.RefreshCachedQueries(ctx, targetInstance)
			}
		}
	}()
}

// List all Redis instances based on the parent string, which is like "projects/{project}/locations/{location}"
func (r *RedisClient) ListRedisInstances(ctx context.Context, parent string) []*rpb.Instance {
	listreq := &rpb.ListInstancesRequest{
		Parent: parent,
	}
	it := r.gcpClient.ListInstances(ctx, listreq)
	instances := []*rpb.Instance{}
	for {
		sklog.Infof("Iterating...")
		resp, err := it.Next()
		if err == iterator.Done {
			sklog.Infof("Iterated all %d Redis instances for %s.", len(instances), parent)
			break
		} else if err != nil {
			sklog.Errorf("Error found in listing Redis instances: %s. Parent: %s", err.Error(), parent)
			break
		}
		sklog.Infof("Appending Redis instance: %s", resp.Name)
		instances = append(instances, resp)
	}
	return instances
}

// Routine to update the cache for skia perf query UI.
func (r *RedisClient) RefreshCachedQueries(ctx context.Context, instance *rpb.Instance) {
	opts := &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", instance.Host, instance.Port),
		Password: "",
	}
	rdb := redis.NewClient(opts)

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Test code while setting up Redis instance.
	currentValue, err := rdb.Get(ctx, "FullPS").Result()
	if err == redis.Nil {
		sklog.Warningf("Key does not exist.")
	} else if err != nil {
		sklog.Errorf("Failed to Get Redis: %s", err.Error())
	} else {
		sklog.Infof("Value read: %s", currentValue)
	}

	err = rdb.Set(ctx, "FullPS", time.Now().Format(time.UnixDate), time.Second*30).Err()
	if err != nil {
		sklog.Errorf("Failed to Set Redis: %s", err.Error())
	}
}
