package rotations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestFromURL(t *testing.T) {
	unittest.SmallTest(t)

	url := "rotations.com/fake"
	test := func(name, content string, expectEmails []string, expectErr string) {
		t.Run(name, func(t *testing.T) {
			urlMock := mockhttpclient.NewURLMock()
			urlMock.MockOnce(url, mockhttpclient.MockGetDialogue([]byte(content)))
			emails, err := FromURL(urlMock.Client(), url)
			if expectErr != "" {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), fmt.Sprintf(errMsgTmpl, url, content))
				require.Contains(t, err.Error(), expectErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, expectEmails, emails)
			}
		})
	}

	test("Invalid JSON", `blahblah`, nil, "invalid character 'b' looking for beginning of value")
	test("Missing reviewer field", `{"otherField": "otherValue"}`, nil, "Missing 'emails' and 'username' field")
	test("Username", `{"username": "me@google.com"}`, []string{"me@google.com"}, "")
	test("Emails", `{"emails": ["me@google.com", "you@google.com"]}`, []string{"me@google.com", "you@google.com"}, "")
	test("UsernameAndEmails", `{"username": "us@google.com", "emails": ["me@google.com", "you@google.com"]}`, []string{"me@google.com", "us@google.com", "you@google.com"}, "")
	test("UnknownKeyOkay", `{"username": "us@google.com", "extraKey": "extraValue"}`, []string{"us@google.com"}, "")
	test("NoDuplicates", `{"username": "us@google.com", "emails": ["us@google.com", "us@google.com"]}`, []string{"us@google.com"}, "")
	test("EmptyEmailsNoError", `{"emails": []}`, []string{}, "")
}
