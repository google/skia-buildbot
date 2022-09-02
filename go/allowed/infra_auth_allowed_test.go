package allowed

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
)

func TestInfraConvert(t *testing.T) {
	infra := []string{
		"user:*@google.com",
		"user:test@example.com",
		"anonymous:anonymous",
		"bot:foo",
		"service:bar",
		"user:",
		"user:last@example.org",
	}

	expected := []string{
		"google.com",
		"test@example.com",
		"last@example.org",
	}

	assert.Equal(t, expected, infraAuthToAllowFromList(infra))
	assert.Equal(t, []string{}, infraAuthToAllowFromList([]string{}))
}

const JSON = `{
  "group": {
    "members": [
      "user:test@example.org",
      "user:*@chromium.org"
    ],
    "nested": [
      "nested_group"
    ],
    "globs": [
      "user:*@gotham.org"
    ]
  }
}`

const NESTED_GROUP_JSON = `{
  "group": {
    "members": [
      "user:nested-user@example.org"
    ]
  }
}`

func TestWithClientMock(t *testing.T) {
	m := mockhttpclient.NewURLMock()
	m.Mock(fmt.Sprintf(GROUP_URL_TEMPLATE, "test"), mockhttpclient.MockGetDialogue([]byte(JSON)))
	m.Mock(fmt.Sprintf(GROUP_URL_TEMPLATE, "nested_group"), mockhttpclient.MockGetDialogue([]byte(NESTED_GROUP_JSON)))
	i, err := NewAllowedFromChromeInfraAuth(m.Client(), "test")
	assert.NoError(t, err)
	assert.True(t, i.Member("foo@chromium.org"))
	assert.True(t, i.Member("test@example.org"))
	assert.True(t, i.Member("nested-user@example.org"))
	assert.True(t, i.Member("batman@gotham.org"))
	assert.False(t, i.Member("example.org"))
	assert.False(t, i.Member("bar@example.org"))
}
