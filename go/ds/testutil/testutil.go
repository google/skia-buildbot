package testutil

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"google.golang.org/api/iterator"
)

type CleanupFunc func()

func cleanup(t assert.TestingT, kinds ...ds.Kind) {
	for _, kind := range kinds {
		q := ds.NewQuery(kind).KeysOnly()
		it := ds.DS.Run(context.TODO(), q)
		for {
			k, err := it.Next(nil)
			if err == iterator.Done {
				break
			} else if err != nil {
				t.Errorf("Failed to clean database: %s", err)
				t.FailNow()
			}
			err = ds.DS.Delete(context.Background(), k)
			assert.NoError(t, err)
		}
	}
}

// InitDatastore is a common utility function used in tests. It sets up the
// datastore to connect to the emulator and also clears out all instances of
// the given 'kinds' from the datastore.
func InitDatastore(t assert.TestingT, kinds ...ds.Kind) CleanupFunc {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	emulatorHost := os.Getenv("DATASTORE_EMULATOR_HOST")
	if emulatorHost == "" {
		assert.Fail(t, `Running tests that require a running Cloud Datastore emulator.

Run

	"gcloud beta emulators datastore start --no-store-on-disk --host-port=localhost:8888"

and then run

  $(gcloud beta emulators datastore env-init)

to set the environment variables. When done running tests you can unset the env variables:

  $(gcloud beta emulators datastore env-unset)

`)
	}

	// Copied from net/http to create a fresh http client. In some tests the
	// httpmock replaces the default http client and the healthcheck below fails.
	var transport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	httpClient := &http.Client{Transport: transport}

	// Do a quick healthcheck against the host, which will fail immediately if it's down.
	_, err := httpClient.Get("http://" + emulatorHost + "/")
	assert.NoError(t, err, fmt.Sprintf("Cloud emulator host %s appears to be down or not accessible.", emulatorHost))

	err = ds.InitForTesting("test-project", fmt.Sprintf("test-namespace-%d", r.Uint64()))
	assert.NoError(t, err)
	cleanup(t, kinds...)
	return func() {
		cleanup(t, kinds...)
	}
}
