package protoheader

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/secret/mocks"
	"go.skia.org/infra/go/testutils"
)

const (
	testEmail      = "fred@example.org"
	testHeaderName = "X-USER"
	testLoginURL   = "https://login.example.org"
)

var (
	errMyMockError = errors.New("authproxy test error")
)

func emailSerializedAsProto(t *testing.T) []byte {
	h := Header{
		Email: testEmail,
	}
	b, err := proto.Marshal(&h)
	require.NoError(t, err)
	return b
}

func protoHeaderMissingAPeriodInTheHeaderValueAndRequestForTest(t *testing.T) (ProtoHeader, *http.Request) {
	b64 := base64.RawURLEncoding.EncodeToString(emailSerializedAsProto(t))
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(testHeaderName, b64) // No ".stuff-added-after-the-dot"
	p := ProtoHeader{
		headerName: testHeaderName,
		loginURL:   testLoginURL,
	}

	return p, r
}

func protoHeaderAndRequestForTest(t *testing.T) (ProtoHeader, *http.Request) {
	b64 := base64.RawURLEncoding.EncodeToString(emailSerializedAsProto(t))
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(testHeaderName, b64+".stuff-added-after-the-dot")
	p := ProtoHeader{
		headerName: testHeaderName,
		loginURL:   testLoginURL,
	}

	return p, r
}

func TestSerialize_HappyPath(t *testing.T) {
	var h2 Header
	err := proto.Unmarshal(emailSerializedAsProto(t), &h2)
	require.NoError(t, err)
	require.Equal(t, testEmail, h2.Email)
}

func TestLoggedInAs_HappyPath(t *testing.T) {
	p, r := protoHeaderAndRequestForTest(t)

	email, err := p.LoggedInAs(r)
	require.NoError(t, err)
	require.Equal(t, testEmail, email)
}

func TestLoggedInAs_MissingPeriodInHash_ReturnsEmptyString(t *testing.T) {
	p, r := protoHeaderMissingAPeriodInTheHeaderValueAndRequestForTest(t)

	email, err := p.LoggedInAs(r)
	require.Equal(t, errDotInHeaderRequired, err)
	require.Empty(t, email)
}

func TestLoggedInAs_HeaderIsMissing_ReturnsEmptyString(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)

	p := ProtoHeader{
		headerName: testHeaderName,
	}
	email, err := p.LoggedInAs(r)
	require.Error(t, err)
	require.Equal(t, "", email)
}

func TestLoggedInAs_HeaderContainsInvalidBase64Encoding_ReturnsEmptyString(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(testHeaderName, "this is not valid base64 encoded text.")

	p := ProtoHeader{
		headerName: testHeaderName,
	}
	email, err := p.LoggedInAs(r)
	require.Contains(t, err.Error(), "decoding base64")
	require.Equal(t, "", email)
}

func TestLoginURL_Always_ReturnsTheSameLoginURL(t *testing.T) {
	p, r := protoHeaderAndRequestForTest(t)

	w := httptest.NewRecorder()
	require.Equal(t, testLoginURL, p.LoginURL(w, r))
}

func TestProtoHeaderInit_AlwaysReturnsNil(t *testing.T) {
	require.Nil(t, ProtoHeader{}.Init(context.Background()))
}

func TestNew_HappyPath(t *testing.T) {
	client := mocks.NewClient(t)
	client.On("Get", testutils.AnyContext, Project, HeaderSecretName, secret.VersionLatest).Return(testHeaderName, nil)
	client.On("Get", testutils.AnyContext, Project, LoginURNSecretName, secret.VersionLatest).Return(testLoginURL, nil)
	got, err := New(context.Background(), client)
	require.NoError(t, err)
	require.Equal(t, testHeaderName, got.headerName)
	require.Equal(t, testLoginURL, got.loginURL)
}

func TestNew_SecretGetReturnsError_ReturnsError(t *testing.T) {
	client := mocks.NewClient(t)
	client.On("Get", testutils.AnyContext, Project, HeaderSecretName, secret.VersionLatest).Return("", errMyMockError)
	_, err := New(context.Background(), client)
	require.Contains(t, err.Error(), errMyMockError.Error())
}
