package secret

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"

	"go.skia.org/infra/go/skerr"
)

const (
	// VersionLatest can be provided to Get() in order to retrieve the most recent
	// version of a secret.
	VersionLatest = "latest"

	// secretAccessorRole is the IAM role given to a service account for access
	// to a specific secret.
	secretAccessorRole = "roles/secretmanager.secretAccessor"

	// serviceAccountPrefix is a prefix applied to service account emails when
	// setting IAM roles.
	serviceAccountPrefix = "serviceAccount:"
)

// Client provides functionality for working with GCP Secrets.
type Client interface {
	// Get the value of the given secret at the given version.  Use "latest" to
	// retrieve the most recent version.
	Get(ctx context.Context, project, secret, version string) (string, error)

	// Update the given secret with a new version. Returns the resulting version.
	Update(ctx context.Context, project, name, value string) (string, error)

	// Describe returns metadata about the secret.
	Describe(ctx context.Context, project, name string) (*secretmanagerpb.Secret, error)

	// Close cleans up resources used by the Client.
	Close() error
}

// ClientImpl implements Client.
type ClientImpl struct {
	client *secretmanager.Client
}

// NewClient returns a Client instance.
func NewClient(ctx context.Context) (*ClientImpl, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create secretmanager client")
	}
	return &ClientImpl{
		client: client,
	}, nil
}

// Get implements the Client interface.
func (c *ClientImpl) Get(ctx context.Context, project, secret, version string) (string, error) {
	// Build the request.
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", project, secret, version),
	}

	// Call the API.
	result, err := c.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to access secret version")
	}
	return string(result.Payload.Data), nil
}

// Update implements the Client interface.
func (c *ClientImpl) Update(ctx context.Context, project, name, value string) (string, error) {
	// Build the request.
	req := &secretmanagerpb.AddSecretVersionRequest{
		Parent: fmt.Sprintf("projects/%s/secrets/%s", project, name),
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(value),
		},
	}

	// Call the API.
	result, err := c.client.AddSecretVersion(ctx, req)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to add secret version")
	}
	split := strings.Split(result.Name, "/")
	return split[len(split)-1], nil
}

// Describe implements the Client interface.
func (c *ClientImpl) Describe(ctx context.Context, project, name string) (*secretmanagerpb.Secret, error) {
	// Build the request.
	req := &secretmanagerpb.GetSecretRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s", project, name),
	}

	// Call the API.
	result, err := c.client.GetSecret(ctx, req)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to access secret")
	}
	return result, nil
}

// Close implements the Client interface.
func (c *ClientImpl) Close() error {
	return skerr.Wrap(c.client.Close())
}

// Assert that ClientImpl implements Client.
var _ Client = &ClientImpl{}
