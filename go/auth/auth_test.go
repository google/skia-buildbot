package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
)

func TestSkolo(t *testing.T) {
	m := mockhttpclient.NewURLMock()
	src := `{"access_token":"ya29.c.El...zwJOP","expires_in":900,"token_type":"Bearer"}`
	m.Mock("http://metadata/computeMetadata/v1/instance/service-accounts/default/token", mockhttpclient.MockGetDialogue([]byte(src)))

	s := skoloTokenSource{
		client: m.Client(),
	}
	token, err := s.Token()
	assert.NoError(t, err)
	assert.True(t, token.Valid())
	assert.True(t, time.Now().Before(token.Expiry))
}
