package sharedb

import (
	"fmt"
	"net"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const MAX_KEYS = 20
const DATA_DIR = "./data_dir"
const SERVER_ADDRESS = "127.0.0.1:9999"

func TestShareDB(t *testing.T) {
	// Create the server and start it.
	serverImpl := NewServer(DATA_DIR)
	defer util.RemoveAll(DATA_DIR)

	grpcServer, client, err := startServer(t, serverImpl)
	assert.Nil(t, err)
	defer grpcServer.Stop()

	dbName := "database001"
	bucketName := "bucket_01"
	ctx := context.Background()

	allKeys := []string{}
	for k := 0; k < MAX_KEYS; k++ {
		key := fmt.Sprintf("key_%4d", k)
		value := fmt.Sprintf("val_%4d", k)

		ack, err := client.Put(ctx, &PutRequest{dbName, bucketName, key, []byte(value)})
		assert.Nil(t, err)
		assert.True(t, ack.Ok)

		foundResp, err := client.Get(ctx, &GetRequest{dbName, bucketName, key})
		assert.Nil(t, err)
		assert.Equal(t, value, string(foundResp.Value))

		allKeys = append(allKeys, key)
	}

	foundDBs, err := client.Databases(ctx, &DatabasesRequest{})
	assert.Nil(t, err)
	assert.Equal(t, []string{dbName}, foundDBs.Values)

	foundBuckets, err := client.Buckets(ctx, &BucketsRequest{dbName})
	assert.Nil(t, err)
	assert.Equal(t, []string{bucketName}, foundBuckets.Values)

	foundKeys, err := client.Keys(ctx, &KeysRequest{dbName, bucketName})
	assert.Nil(t, err)

	sort.Strings(foundKeys.Values)
	sort.Strings(allKeys)
	assert.Equal(t, allKeys, foundKeys.Values)

	for _, k := range foundKeys.Values {
		ack, err := client.Delete(ctx, &DeleteRequest{dbName, bucketName, k})
		assert.Nil(t, err)
		assert.True(t, ack.Ok)

		foundVal, err := client.Get(ctx, &GetRequest{dbName, bucketName, k})
		assert.Nil(t, err)
		assert.Nil(t, foundVal.Value)
	}

	foundKeys, err = client.Keys(ctx, &KeysRequest{dbName, bucketName})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(foundKeys.Values))
}

func startServer(t *testing.T, serverImpl ShareDBServer) (*grpc.Server, ShareDBClient, error) {
	lis, err := net.Listen("tcp", SERVER_ADDRESS)
	assert.Nil(t, err)
	grpcServer := grpc.NewServer()
	RegisterShareDBServer(grpcServer, serverImpl)
	go func() { _ = grpcServer.Serve(lis) }()

	// 10ms should be plenty to bring up the server on the loopback interface
	// and connect to it.
	for i := 0; i < 10; i++ {
		client, err := New(SERVER_ADDRESS)
		if err == nil {
			return grpcServer, client, nil
		}
		fmt.Printf("\n\nError: %s\n", err)
		time.Sleep(time.Millisecond)
	}

	return nil, nil, fmt.Errorf("Unable to connect to server.")
}
