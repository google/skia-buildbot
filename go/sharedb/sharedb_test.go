package sharedb

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const MAX_KEYS = 200
const DATA_DIR = "./data_dir"

func TestShareDB(t *testing.T) {
	testutils.LargeTest(t)
	// Create the server and start it.
	serverImpl := NewServer(DATA_DIR)
	defer util.RemoveAll(DATA_DIR)

	grpcServer, client, err := startServer(t, serverImpl)
	assert.NoError(t, err)
	defer grpcServer.Stop()
	defer func() { assert.NoError(t, client.Close()) }()

	dbName := "database001"
	bucketName := "bucket_01"
	ctx := context.Background()

	allKeys := []string{}
	for k := 0; k < MAX_KEYS; k++ {
		key := fmt.Sprintf("key_%04d", k)
		value := fmt.Sprintf("val_%04d", k)

		ack, err := client.Put(ctx, &PutRequest{dbName, bucketName, key, []byte(value)})
		assert.NoError(t, err)
		assert.True(t, ack.Ok)

		foundResp, err := client.Get(ctx, &GetRequest{dbName, bucketName, key})
		assert.NoError(t, err)
		assert.Equal(t, value, string(foundResp.Value))

		allKeys = append(allKeys, key)
	}

	foundDBs, err := client.Databases(ctx, &DatabasesRequest{})
	assert.NoError(t, err)
	assert.Equal(t, []string{dbName}, foundDBs.Values)

	foundBuckets, err := client.Buckets(ctx, &BucketsRequest{dbName})
	assert.NoError(t, err)
	assert.Equal(t, []string{bucketName}, foundBuckets.Values)

	foundKeys, err := client.Keys(ctx, &KeysRequest{dbName, bucketName, "", "", ""})
	assert.NoError(t, err)

	sort.Strings(foundKeys.Values)
	sort.Strings(allKeys)
	assert.Equal(t, allKeys, foundKeys.Values)

	// Test a min-max range scan.
	foundKeys, err = client.Keys(ctx, &KeysRequest{dbName, bucketName, "", "key_0010", "key_0015"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"key_0010", "key_0011", "key_0012", "key_0013", "key_0014", "key_0015"}, foundKeys.Values)

	// Test a min to end range scan.
	foundKeys, err = client.Keys(ctx, &KeysRequest{dbName, bucketName, "", "key_0015", ""})
	assert.NoError(t, err)
	assert.Equal(t, []string{"key_0015", "key_0016", "key_0017", "key_0018", "key_0019"}, foundKeys.Values[0:5])

	// Test a start to max range scan.
	foundKeys, err = client.Keys(ctx, &KeysRequest{dbName, bucketName, "", "", "key_0004"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"key_0000", "key_0001", "key_0002", "key_0003", "key_0004"}, foundKeys.Values)

	// Test a prefix scan.
	foundKeys, err = client.Keys(ctx, &KeysRequest{dbName, bucketName, "key_000", "", ""})
	assert.NoError(t, err)
	exp := []string{"key_0000", "key_0001", "key_0002", "key_0003", "key_0004",
		"key_0005", "key_0006", "key_0007", "key_0008", "key_0009"}
	assert.Equal(t, exp, foundKeys.Values)

	for _, k := range allKeys {
		ack, err := client.Delete(ctx, &DeleteRequest{dbName, bucketName, k})
		assert.NoError(t, err)
		assert.True(t, ack.Ok)

		foundVal, err := client.Get(ctx, &GetRequest{dbName, bucketName, k})
		assert.NoError(t, err)
		assert.Nil(t, foundVal.Value)
	}

	foundKeys, err = client.Keys(ctx, &KeysRequest{dbName, bucketName, "", "", ""})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(foundKeys.Values))

	foundResp, err := client.Get(ctx, &GetRequest{"Does-not-exist-either", bucketName, "does-not-exist"})
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "Does-not-exist-either"))

	foundResp, err = client.Get(ctx, &GetRequest{dbName, bucketName, "does-not-exist"})
	assert.NoError(t, err)
	assert.Equal(t, "", string(foundResp.Value))
}

func startServer(t *testing.T, serverImpl ShareDBServer) (*grpc.Server, *ShareDB, error) {
	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)
	grpcServer := grpc.NewServer()
	RegisterShareDBServer(grpcServer, serverImpl)
	go func() { _ = grpcServer.Serve(lis) }()

	// 10ms should be plenty to bring up the server on the loopback interface
	// and connect to it.
	for i := 0; i < 10; i++ {
		client, err := New(lis.Addr().String())
		if err == nil {
			return grpcServer, client, nil
		}
		fmt.Printf("\n\nError: %s\n", err)
		time.Sleep(time.Millisecond)
	}

	return nil, nil, fmt.Errorf("Unable to connect to server.")
}
