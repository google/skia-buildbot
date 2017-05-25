package storage

import (
	"fmt"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/golden/go/mocks"
)

const (
	// TEST_BUCKET_NAME is the bucket where the test file will be written.
	TEST_BUCKET_NAME = "skia-infra-testdata"

	// TEST_HASH_FILE_PATH is the path to the test hash file.
	TEST_HASH_FILE_PATH = "hash_files/testing-known-hashes.txt"
)

func TestGStorageClient(t *testing.T) {
	testutils.MediumTest(t)

	client := mocks.GetHTTPClient(t)
	timeStamp := fmt.Sprintf("%032d", time.Now().UnixNano())
	gsClient, err := NewGStorageClient(client, TEST_BUCKET_NAME, TEST_HASH_FILE_PATH+"-"+timeStamp)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, gsClient.RemoveKownDigests())
	}()

	knownDigests := []string{
		"c003788f8d306ff1226e2a460835dae4",
		"885b31941c25efc313b0fd66d55b86d9",
		"264d0d87b12ba337f796fc592cd5357d",
		"69c2fbf8e89a48058b2f45ad4ea46a35",
		"2c4d605c16e7d5b23294c0433fa3ed17",
		"782717cf6ed9329fc43cb5a6c830cbce",
		"e143ca619f2172d06bb0dcc4d72af414",
		"26aff0619c829bc149f7c0171fcca442",
		"72d61ae8e232c3a279cc3cdbf6ef73e5",
		"f1eb049dac1cfa3c70aac8fc6ad5496f",
		timeStamp,
	}
	assert.NoError(t, gsClient.WriteKownDigests(knownDigests))

	found, err := gsClient.LoadKownDigests()
	assert.NoError(t, err)
	assert.Equal(t, knownDigests, found)
}
