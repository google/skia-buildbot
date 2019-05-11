package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	tokenUrl = "http://metadata/computeMetadata/v1/instance/service-accounts/default/token"
)

func TestSkolo(t *testing.T) {
	unittest.SmallTest(t)
	m := mockhttpclient.NewURLMock()
	src := `{"access_token":"ya29.c.El...zwJOP","expires_in":900,"token_type":"Bearer"}`
	m.Mock(tokenUrl, mockhttpclient.MockGetDialogue([]byte(src)))

	s := skoloTokenSource{
		client: m.Client(),
	}
	token, err := s.Token()
	assert.NoError(t, err)
	assert.True(t, token.Valid())
	assert.Equal(t, "Bearer", token.TokenType)
	assert.True(t, time.Now().Before(token.Expiry))
}

func TestSkoloFail(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		value    string
		hasError bool
		message  string
	}{
		{
			value:    `{"access_token":"ya29.c.El...zwJOP","expires_in":900,"token_type":"Bearer"}`,
			hasError: false,
			message:  "Good",
		},
		{
			value:    `{"access_token":"ya29.c.El...zwJOP","token_type":"Bearer"}`,
			hasError: true,
			message:  "Missing expires_in.",
		},
		{
			value:    `{"expires_in":900,"token_type":"Bearer"}`,
			hasError: true,
			message:  "Missing access_token.",
		},
	}

	for _, tc := range testCases {
		m := mockhttpclient.NewURLMock()
		m.Mock(tokenUrl, mockhttpclient.MockGetDialogue([]byte(tc.value)))
		s := skoloTokenSource{
			client: m.Client(),
		}
		_, err := s.Token()
		if got, want := (err != nil), tc.hasError; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
