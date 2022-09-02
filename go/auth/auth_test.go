package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
)

const (
	tokenUrl = "http://metadata/computeMetadata/v1/instance/service-accounts/default/token"
)

func TestSkoloToken_ValidToken_Success(t *testing.T) {

	const fakeToken = `{"access_token":"ya29.c.El...zwJOP","expires_in":900,"token_type":"Bearer"}`

	m := mockhttpclient.NewURLMock()
	m.Mock(tokenUrl, mockhttpclient.MockGetDialogue([]byte(fakeToken)))
	s := skoloTokenSource{
		client: m.Client(),
	}
	token, err := s.Token()
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.True(t, token.Valid())
	assert.Equal(t, "Bearer", token.TokenType)
	assert.True(t, time.Now().Before(token.Expiry))
}

func TestSkoloToken_InvalidToken_ReturnsError(t *testing.T) {

	test := func(name, token string) {
		t.Run(name, func(t *testing.T) {
			m := mockhttpclient.NewURLMock()
			m.Mock(tokenUrl, mockhttpclient.MockGetDialogue([]byte(token)))
			s := skoloTokenSource{
				client: m.Client(),
			}
			_, err := s.Token()
			require.Error(t, err)
		})
	}

	test("Missing expires_in", `{"access_token":"ya29.c.El...zwJOP","token_type":"Bearer"}`)
	test("Missing access_token", `{"expires_in":900,"token_type":"Bearer"}`)
}
