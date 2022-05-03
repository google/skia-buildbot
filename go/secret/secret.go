package secret

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"

	"go.skia.org/infra/go/skerr"
)

const (
	// VersionLatest can be provided to Get() in order to retrieve the most recent
	// version of a secret.
	VersionLatest = "latest"

	// secretAccessorRole is the IAM role given to a service account for access
	// to a specific secret.
	secretAccessorRole = "roles/secretmanager.secretAccessor"
)

// Client provides functionality for working with GCP Secrets.
type Client interface {
	// Get the value of the given secret at the given version.  Use "latest" to
	// retrieve the most recent version.
	Get(ctx context.Context, project, secret, version string) (string, error)

	// Update the given secret with a new version. Returns the resulting version.
	Update(ctx context.Context, project, name, value string) (string, error)

	// Create a new secret in the given project with the given name.
	Create(ctx context.Context, project, name string) error

	// GrantAccess grants the given service account access to the given secret.
	GrantAccess(ctx context.Context, project, name, serviceAccount string) error

	// RevokeAccess revokes the given service account's access to the given
	// secret.
	RevokeAccess(ctx context.Context, project, name, serviceAccount string) error

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

// Create implements the Client interface.
func (c *ClientImpl) Create(ctx context.Context, project, name string) error {
	// Build the request.
	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", project),
		SecretId: name,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}

	// Call the API.
	if _, err := c.client.CreateSecret(ctx, req); err != nil {
		return skerr.Wrapf(err, "failed to create secret")
	}

	return nil
}

// Close implements the Client interface.
func (c *ClientImpl) Close() error {
	return skerr.Wrap(c.client.Close())
}

// GrantAccess implements the Client interface.
func (c *ClientImpl) GrantAccess(ctx context.Context, project, name, serviceAccount string) error {
	policyHandle := c.client.IAM(fmt.Sprintf("projects/%s/secrets/%s", project, name))
	policy, err := policyHandle.Policy(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve IAM policy")
	}
	policy.Add(serviceAccount, secretAccessorRole)
	if err := policyHandle.SetPolicy(ctx, policy); err != nil {
		return skerr.Wrapf(err, "failed to set IAM policy")
	}
	return nil
}

// RevokeAccess implements the Client interface.
func (c *ClientImpl) RevokeAccess(ctx context.Context, project, name, serviceAccount string) error {
	policyHandle := c.client.IAM(fmt.Sprintf("projects/%s/secrets/%s", project, name))
	policy, err := policyHandle.Policy(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to retrieve IAM policy")
	}
	policy.Remove(serviceAccount, secretAccessorRole)
	if err := policyHandle.SetPolicy(ctx, policy); err != nil {
		return skerr.Wrapf(err, "failed to set IAM policy")
	}
	return nil
}

// Assert that ClientImpl implements Client.
var _ Client = &ClientImpl{}
