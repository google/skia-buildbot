// Package header supports extracting the email of an authorized user from a
// protobuf in an HTTP Header.
package protoheader

import (
	"context"
	"encoding/base64"
	"net/http"

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
func (p ProtoHeader) Init(port string, local bool) error {
	return nil
}

// LoggedInAs implements auth.Auth.
func (p ProtoHeader) LoggedInAs(r *http.Request) string {
	b, err := base64.StdEncoding.DecodeString(r.Header.Get(p.headerName))
	if err != nil {
		sklog.Errorf("Failed to base64 decode header %q: %s", p.headerName, err)
		return ""
	}
	var h Header
	err = proto.Unmarshal(b, &h)
	if err != nil {
		sklog.Errorf("Failed to decode proto %q: %s", p.headerName, err)
		return ""
	}
	return h.Email
}

// LoginURL implements auth.Auth.
func (p ProtoHeader) LoginURL(w http.ResponseWriter, r *http.Request) string {
	return p.loginURL
}

// Confirm we implement the interface.
var _ auth.Auth = ProtoHeader{}
