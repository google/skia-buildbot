package bbtrybot

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/sklog"
	gstorage "google.golang.org/api/storage/v1"
)

func TestTrybotStatus(t *testing.T) {
	// Get the client to be used to access GCS and the Monorail issue tracker.
	serviceAccountFile := "./service-account.json"
	client, err := auth.NewJWTServiceAccountClient("", serviceAccountFile, nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	// oauthCacheFile := "./google_storage_token.data"
	// httpClient, err := auth.NewClient(true, oauthCacheFile, auth.SCOPE_READ_WRITE)
	// if err != nil {
	// 	sklog.Fatal(err)
	// }

	tbStatus, err := NewTrybotStatus(client, nil)
	assert.NoError(t, err)

	tbStatus.PollBuildBucket()
}
