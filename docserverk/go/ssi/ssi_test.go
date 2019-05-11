package ssi

import (
	"bytes"
	"fmt"
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestProcessSSI(t *testing.T) {
	unittest.SmallTest(t)

	// Inject our test function below.
	processFNs["testfn"] = getTestProcessFn(t, "testfn")
	processFNs["errfn"] = func(map[string]string) ([]byte, error) {
		return nil, fmt.Errorf("Error!")
	}

	checkSSI(t, "<p><ssi:testfn k1=v2></p>", "<p>testfn:k1:v2:</p>")
	checkSSI(t, "<p><ssi:testfn k1=v2 k2=v2></p>", "<p>testfn:k1:v2:k2:v2:</p>")
	checkSSI(t, "<p><ssi:testfn></p>", "<p>testfn:</p>")

	_, err := ProcessSSI([]byte("<p><ssi:invalidfn k1=v1></p>"))
	assert.Error(t, err)
	_, err = ProcessSSI([]byte("<p><ssi:testfn =v1></p>"))
	assert.Error(t, err)
	_, err = ProcessSSI([]byte("<p><ssi:errfn k1=v1></p>"))
	assert.Error(t, err)
}

const (
	// Alternate row template to make testing easier.
	testRowTmpl = "{{.created}}:{{.url}}:{{.name}}:{{.date}}:{{.commit}}:{{.commit_url}}:{{.commit_message}}\n"

	// Expected output.
	expOutput = `2018-03-05T12:44:24Z:https://storage.cloud.google.com/skia-infra-testdata/skqp-testing/skqp-universal-006-debug.apk:skqp-universal-006-debug.apk: 2018-03-05T12:32:58Z:95a7b76a44edd2f25423a4d395df553b80fe06d7:https://example.com/+/95a7b76a44edd2f25423a4d395df553b80fe06d7:Add new feature to dm
2018-03-05T12:44:17Z:https://storage.cloud.google.com/skia-infra-testdata/skqp-testing/skqp-universal-005-debug.apk:skqp-universal-005-debug.apk: 2018-03-05T12:32:58Z:95a7b76a44edd2f25423a4d395df553b80fe06d7:https://example.com/+/95a7b76a44edd2f25423a4d395df553b80fe06d7:Add new feature to dm
2018-03-05T12:44:09Z:https://storage.cloud.google.com/skia-infra-testdata/skqp-testing/skqp-universal-004-debug.apk:skqp-universal-004-debug.apk: 2018-03-05T12:32:58Z:95a7b76a44edd2f25423a4d395df553b80fe06d7:https://example.com/+/95a7b76a44edd2f25423a4d395df553b80fe06d7:Add new feature to dm
2018-03-05T12:44:01Z:https://storage.cloud.google.com/skia-infra-testdata/skqp-testing/skqp-universal-003-debug.apk:skqp-universal-003-debug.apk: 2018-03-05T12:32:58Z:95a7b76a44edd2f25423a4d395df553b80fe06d7:https://example.com/+/95a7b76a44edd2f25423a4d395df553b80fe06d7:Add new feature to dm
2018-03-05T12:32:53Z:https://storage.cloud.google.com/skia-infra-testdata/skqp-testing/skqp-universal-002-debug.apk:skqp-universal-002-debug.apk: 2018-03-05T12:32:58Z:95a7b76a44edd2f25423a4d395df553b80fe06d7:https://example.com/+/95a7b76a44edd2f25423a4d395df553b80fe06d7:Add new feature to dm`
)

/* Test assumes a local JWT service account.
func TestGCEListing(t *testing.T) {

	unittest.MediumTest(t)

	// Swap out the templates to make testing easier.
	listGCSTagSnippet = "%s"
	gceListingRowTmpl = template.Must(template.New("row").Parse(testRowTmpl))

	tokenSrc, err := auth.NewJWTServiceAccountTokenSource("", "", storage.ScopeFullControl)
	assert.NoError(t, err)

	client, err := storage.NewClient(context.Background(), option.WithTokenSource(tokenSrc))
	assert.NoError(t, err)

	Init("https://example.com", client)
	doc, err := ProcessSSI([]byte("<ssi:listgce path=skia-infra-testdata/skqp-testing>"))
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, expOutput, strings.TrimSpace(string(doc)))
}
*/

func checkSSI(t *testing.T, tag, expDoc string) {
	doc, err := ProcessSSI([]byte(tag))
	assert.NoError(t, err)
	assert.Equal(t, expDoc, string(doc))
}

func getTestProcessFn(t *testing.T, tagName string) ssiProcessFn {
	return func(params map[string]string) ([]byte, error) {
		// Write the keys in alphabetical order.
		keys := []string{}
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var buf bytes.Buffer
		buf.Write([]byte(tagName + ":"))
		for _, k := range keys {
			_, err := buf.Write([]byte(fmt.Sprintf("%s:%s:", k, params[k])))
			assert.NoError(t, err)
		}
		return buf.Bytes(), nil
	}
}
