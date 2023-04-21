package docker

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	manifestURLTemplate = "https://%s/v2/%s/manifests/%s"
	acceptHeader        = "Accept"
	acceptContentType   = "application/vnd.docker.distribution.manifest.v2+json"
	digestHeader        = "Docker-Content-Digest"
)

// GetDigest retrieves the digest for the given image from the registry.
func GetDigest(ctx context.Context, client *http.Client, registry, repository, tag string) (string, error) {
	url := fmt.Sprintf(manifestURLTemplate, registry, repository, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	req.Header.Set(acceptHeader, acceptContentType)
	resp, err := client.Do(req)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	vals := resp.Header[digestHeader]
	if len(vals) != 1 {
		return "", skerr.Fmt("found %d values for %s header", len(vals), digestHeader)
	}
	return vals[0], nil
}
