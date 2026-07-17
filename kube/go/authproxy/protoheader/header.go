// Package header supports extracting the email of an authorized user from a
// protobuf in an HTTP Header.
package protoheader

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy/auth"
	"google.golang.org/protobuf/proto"
)

const (
	// HeaderSecretName is the name of the GCP secret for login.
	HeaderSecretName = "authproxy-protoheader-name"

	// LoginURNSecretName is the name of the GCP secret for the login URL.
	LoginURNSecretName = "authproxy-loginurl"

	// Project is the project where the above secrets are stored in.
	Project = "skia-infra-public"
)

var (
	errDotInHeaderRequired = errors.New("Failed to find a '.' separated header value.")
)

// ProtoHeader implements auth.Auth.
type ProtoHeader struct {
	headerName string
	loginURL   string
}

// New creates a ProtoHeader.
func New(ctx context.Context, secretClient secret.Client) (ProtoHeader, error) {
	var ret ProtoHeader
	headerName, err := secretClient.Get(ctx, Project, HeaderSecretName, secret.VersionLatest)
	if err != nil {
		return ret, skerr.Wrapf(err, "failed loading secrets from GCP secret manager; failed to retrieve secret %q", HeaderSecretName)
	}
	ret.headerName = headerName

	loginURL, err := secretClient.Get(ctx, Project, LoginURNSecretName, secret.VersionLatest)
	if err != nil {
		return ret, skerr.Wrapf(err, "failed loading secrets from GCP secret manager; failed to retrieve secret %q", LoginURNSecretName)
	}
	ret.loginURL = loginURL

	return ret, nil
}

// Init implements auth.Auth.
func (p ProtoHeader) Init(ctx context.Context) error {
	return nil
}

func (p ProtoHeader) LoggedInAs(r *http.Request) (string, error) {
	headerName := strings.TrimSpace(p.headerName)
	headerValue := getHeaderCaseInsensitive(r, headerName)
	if headerValue == "" {
		sklog.Debugf("ProtoHeader: Header %q not found in request headers", headerName)
		return "", skerr.Fmt("header %q missing", headerName)
	}
	parts := strings.Split(headerValue, ".")
	if len(parts) != 2 {
		sklog.Debugf("ProtoHeader: Header %q value %q does not contain signature period separator", headerName, headerValue)
		return "", errDotInHeaderRequired
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		sklog.Debugf("ProtoHeader: Header %q base64 decode error: %v", headerName, err)
		return "", skerr.Wrapf(err, "decoding base64 header: %q", headerName)
	}
	var h Header
	err = proto.Unmarshal(b, &h)
	if err != nil {
		sklog.Debugf("ProtoHeader: Header %q proto unmarshal error: %v", headerName, err)
		return "", skerr.Wrapf(err, "decoding proto %q", headerName)
	}
	return h.Email, nil
}

// LoginURL implements auth.Auth.
func (p ProtoHeader) LoginURL(w http.ResponseWriter, r *http.Request) string {
	return p.loginURL
}

// Confirm we implement the interface.
var _ auth.Auth = ProtoHeader{}

// getHeaderCaseInsensitive performs a robust header lookup.
// It first attempts standard MIME-canonical lookup via r.Header.Get(name).
// If that returns empty (e.g. when HTTP/2 protocol frames deliver lowercased keys
// like "x-endpoint-api-userinfo"), it falls back to a case-insensitive iteration
// over the raw request header map.
func getHeaderCaseInsensitive(r *http.Request, name string) string {
	// 1. Try standard canonical lookup first
	if val := r.Header.Get(name); val != "" {
		return val
	}

	// 2. Fallback: Search the raw map keys case-insensitively using strings.ToLower
	lowerName := strings.ToLower(name)
	for k, v := range r.Header {
		if strings.ToLower(k) == lowerName && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}
