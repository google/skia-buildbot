package ssi

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/storage"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"google.golang.org/api/option"
)

const (
	TEST_DOC_TMPL = `
	<h1>Example<h1>
	%s
	<p>more content</p2>
	`
)

func TestProcessSSI(t *testing.T) {
	// Inject our test function below.
	processFNs["testssi"] = testProcessFN

	// testDoc := []byte(fmt.Sprintf(TEST_DOC_TMPL, "<ssi:listgce path=bucket/path>"))
	testDoc := []byte(fmt.Sprintf(TEST_DOC_TMPL, "<ssi:testssi path=bucket/path>"))
	expDoc := []byte(fmt.Sprintf(TEST_DOC_TMPL, "supercat:path:bucket/path"))

	newDoc := ProcessSSI(testDoc)
	assert.Equal(t, string(expDoc), string(newDoc))
}

func testProcessFN(params map[string]string) []byte {
	for k, v := range params {
		return []byte(fmt.Sprintf("supercat:%s:%s", k, v))
	}
	return nil
}

func TestGCEListing(t *testing.T) {
	tokenSrc, err := auth.NewJWTServiceAccountTokenSource("", "./service-account.json", storage.ScopeFullControl)
	assert.NoError(t, err)

	opts := []option.ClientOption{
		option.WithTokenSource(tokenSrc),
		//		option.WithHTTPClient(httputils.NewTimeoutClient()),
	}
	client, err := storage.NewClient(context.Background(), opts...)
	assert.NoError(t, err)

	Init(client)
	docs := ProcessSSI([]byte("<ssi:listgce path=skia-stephana-test/testing>"))
	assert.NotNil(t, docs)
	fmt.Println(string(docs))
}
