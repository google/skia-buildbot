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
	"errors"
	"fmt"
	"strings"
	"time"

	gcp_redis "cloud.google.com/go/redis/apiv1"
	rpb "cloud.google.com/go/redis/apiv1/redispb"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"google.golang.org/api/iterator"
)

// RedisWrapper define the interfaces for Redis related operations
type RedisWrapper interface {
	StartRefreshRoutine(context.Context, time.Duration, *config.InstanceConfig) error
	ListRedisInstances(context.Context, string) []*rpb.Instance
}

// RedisClient implements RedisWrapper
type RedisClient struct {
	gcpClient *gcp_redis.CloudRedisClient
}

// Create a new Redisclient.
func NewRedisClient(ctx context.Context) (*RedisClient, error) {
	gcpClient, err := gcp_redis.NewCloudRedisClient(ctx)
	if err != nil {
		sklog.Errorf("Cannot create Redis client for Google Cloud.")
		return nil, err
	}
	return &RedisClient{
		gcpClient: gcpClient,
	}, nil
}

// Start a routine to periodically access the Redis instance.
func (r *RedisClient) StartRefreshRoutine(ctx context.Context, refreshPeriod time.Duration, config *config.RedisConfig) error {
	project := config.Project
	zone := config.Zone
	if project == "" || zone == "" {
		sklog.Errorf("No project or zone defined in redis config.")
		return errors.New("empty project or zone")
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", project, zone)
	sklog.Infof("Start listing Redis instances for %s.", parent)
	go func() {
		for range time.Tick(refreshPeriod) {
			var sb strings.Builder
			instances := r.ListRedisInstances(ctx, parent)
			sb.WriteString(fmt.Sprintf("Found %d Redis instances.\n", len(instances)))
			for _, instance := range instances {
				sb.WriteString(fmt.Sprintf("Name: %s, Host: %s, Port: %d\n", instance.Name, instance.Host, instance.Port))
			}
			sklog.Infof(sb.String())
		}
	}()
	return nil
}

// List all Redis instances based on the parent string, which is like "projects/{project}/locations/{location}"
func (r *RedisClient) ListRedisInstances(ctx context.Context, parent string) []*rpb.Instance {
	listreq := &rpb.ListInstancesRequest{
		Parent: parent,
	}
	it := r.gcpClient.ListInstances(ctx, listreq)
	instances := []*rpb.Instance{}
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			sklog.Infof("Iterated all %d Redis instances for %s.", len(instances), parent)
			break
		}
		instances = append(instances, resp)
	}
	return instances
}
