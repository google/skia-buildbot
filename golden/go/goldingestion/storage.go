package goldingestion

import (
	"fmt"
	"strconv"

	"go.skia.org/infra/go/sharedb"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
)

const (
	DB_NAME     = "ingestion-metadata"
	BUCKET_NAME = "ingested"
)

type IngestionStore struct {
	client *sharedb.ShareDB
	ctx    context.Context
}

// TODO(stephana): The meta data store needs a background process to remove meta data we are no longer
// interested in.

// NewIngestionStore allows to store and retrieve meta data about the ingestion process.
// serverAddr specifies the sharedb server. ingesterID is the unique id of the ingester,
// it will be used to prefix database names.
func NewIngestionStore(serverAddr string) (*IngestionStore, error) {
	client, err := sharedb.New(serverAddr)

	if err != nil {
		return nil, err
	}

	return &IngestionStore{
		client: client,
		ctx:    context.Background(),
	}, nil
}

// Close the connection to the RPC service.
func (i *IngestionStore) Close() error {
	return i.client.Close()
}

// IsIngested returns true if the results for the given master/builder/buildnumber have been processed.
func (i *IngestionStore) IsIngested(ingesterID, master, builder string, buildNumber int64) bool {
	resp, err := i.client.Get(i.ctx, &sharedb.GetRequest{Database: DB_NAME, Bucket: getBucketName(ingesterID), Key: getKey(master, builder, buildNumber)})
	if err != nil {
		sklog.Errorf("Error querying ingestion store: %s", err)
		return false
	}
	return resp.Value != nil
}

// Add adds the given master/builder/buildNumber with a timestamp to the data store.
func (i *IngestionStore) Add(ingesterID, master, builder string, buildNumber int64) error {
	value := []byte(strconv.FormatInt(util.TimeStampMs(), 10))
	_, err := i.client.Put(i.ctx, &sharedb.PutRequest{Database: DB_NAME, Bucket: getBucketName(ingesterID), Key: getKey(master, builder, buildNumber), Value: value})
	return err
}

func getBucketName(ingesterID string) string {
	return BUCKET_NAME + "-" + ingesterID
}

func getKey(master, builder string, buildNumber int64) string {
	return fmt.Sprintf("%s:%s:%010d", master, builder, buildNumber)
}
