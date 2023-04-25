package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	blobURLTemplate     = "https://%s/v2/%s/blobs/%s"
	catalogURLTemplate  = "https://%s/v2/_catalog?n=%d"
	listTagsURLTemplate = "https://%s/v2/%s/tags/list?n=%d"
	manifestURLTemplate = "https://%s/v2/%s/manifests/%s"
	acceptHeader        = "Accept"
	acceptContentType   = "application/vnd.docker.distribution.manifest.v2+json"
	digestHeader        = "Docker-Content-Digest"
	pageSize            = 100
)

// Client is used for interacting with a Docker registry.
type Client interface {
	// GetManifest retrieves the manifest for the given image. The reference may
	// be a tag or a digest.
	GetManifest(ctx context.Context, repository, reference string) (*Manifest, error)
	// GetConfig retrieves an image config based on the config.digest from its
	// Manifest.
	GetConfig(ctx context.Context, repository, configDigest string) (*ImageConfig, error)
	// ListRepositories lists all repositories in the given registry. Because there
	// may be many results, this can be quite slow.
	ListRepositories(ctx context.Context) ([]string, error)
	// ListInstances lists all image instances for the given repository, keyed by
	// their digests. Because there may be many results, this can be quite slow.
	ListInstances(ctx context.Context, repository string) (map[string]*ImageInstance, error)
	// ListTags lists all known tags for the given repository. Because there may be
	// many results, this can be quite slow.
	ListTags(ctx context.Context, repository string) ([]string, error)
}

// ClientImpl implements Client.
type ClientImpl struct {
	client   *http.Client
	registry string
}

// NewClient returns a Client instance which interacts with the given registry.
func NewClient(ctx context.Context, client *http.Client, registry string) *ClientImpl {
	return &ClientImpl{
		client:   client,
		registry: registry,
	}
}

type MediaConfig struct {
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Digest    string `json:"digest"`
}

// Manifest represents a Docker image manifest.
type Manifest struct {
	Digest string        `json:"-"`
	Config MediaConfig   `json:"config"`
	Layers []MediaConfig `json:"layers"`
}

// GetManifest implements Client.
func (c *ClientImpl) GetManifest(ctx context.Context, repository, tag string) (*Manifest, error) {
	url := fmt.Sprintf(manifestURLTemplate, c.registry, repository, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	req.Header.Set(acceptHeader, acceptContentType)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	var rv Manifest
	if err := json.NewDecoder(resp.Body).Decode(&rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	digestVals := resp.Header[digestHeader]
	if len(digestVals) != 1 {
		return nil, skerr.Fmt("found %d values for %s header", len(digestVals), digestHeader)
	}
	rv.Digest = digestVals[0]
	return &rv, nil
}

// paginatedResults has the necessary field, "Next", needed to paginate requests
// to the API as well as all of the fields used by callers of "paginate".
type paginatedResults struct {
	Next         string                           `json:"next"`
	Repositories []string                         `json:"repositories"`
	Manifest     map[string]*decodedImageInstance `json:"manifest"`
	Tags         []string                         `json:"tags"`
}

// paginate is a helper function for paginating API requests.
func (c *ClientImpl) paginate(ctx context.Context, url string, cb func(paginatedResults) error) error {
	var header string
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return skerr.Wrap(err)
		}
		if header != "" {
			req.Header.Set("Link", header)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return skerr.Wrap(err)
		}
		var results paginatedResults
		err = json.NewDecoder(resp.Body).Decode(&results)
		util.Close(resp.Body)
		if err != nil {
			return skerr.Wrap(err)
		}
		if err := cb(results); err != nil {
			return skerr.Wrap(err)
		}
		if results.Next != "" {
			url = results.Next
			header = fmt.Sprintf(`<%s>; rel="next"`, url)
		} else {
			return nil
		}
	}
}

// ListRepositories implements Client.
func (c *ClientImpl) ListRepositories(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf(catalogURLTemplate, c.registry, pageSize)
	var rv []string
	if err := c.paginate(ctx, url, func(results paginatedResults) error {
		rv = append(rv, results.Repositories...)
		return nil
	}); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// ImageInstance describes one instance of an image.
type ImageInstance struct {
	Digest    string    `json:"digest"`
	SizeBytes int64     `json:"imageSizeBytes"`
	Tags      []string  `json:"tag"`
	Created   time.Time `json:"timeCreatedMs"`
	Uploaded  time.Time `json:"timeUploadedMs"`
}

// decodedImageInstance is used for parsing an ImageInstance from JSON.
type decodedImageInstance struct {
	ImageSizeBytes string   `json:"imageSizeBytes"`
	Tags           []string `json:"tag"`
	TimeCreatedMs  string   `json:"timeCreatedMs"`
	TimeUploadedMs string   `json:"timeUploadedMs"`
}

// imageInstance creates an ImageInstance from this decodedImageInstance.
func (i decodedImageInstance) imageInstance(digest string) (*ImageInstance, error) {
	sizeBytes, err := strconv.ParseInt(i.ImageSizeBytes, 10, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	createdMs, err := strconv.ParseInt(i.TimeCreatedMs, 10, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	uploadedMs, err := strconv.ParseInt(i.TimeUploadedMs, 10, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &ImageInstance{
		Digest:    digest,
		SizeBytes: sizeBytes,
		Tags:      i.Tags,
		Created:   time.UnixMilli(createdMs),
		Uploaded:  time.UnixMilli(uploadedMs),
	}, nil
}

// ImageInstanceSlice implements sort.Interface.
type ImageInstanceSlice []*ImageInstance

// Less implements sort.Interface.
func (s ImageInstanceSlice) Less(i, j int) bool {
	// First, sort by Created timestamp.
	if s[i].Created.Before(s[j].Created) {
		return true
	} else if s[i].Created.After(s[j].Created) {
		return false
	}
	// Next, sort by Uploaded timestamp.
	if s[i].Uploaded.Before(s[j].Uploaded) {
		return true
	} else if s[i].Uploaded.After(s[j].Uploaded) {
		return false
	}
	// Next, sort by size.
	if s[i].SizeBytes < s[j].SizeBytes {
		return true
	} else if s[i].SizeBytes > s[j].SizeBytes {
		return false
	}
	// Finally, sort by digest.
	return s[i].Digest < s[j].Digest
}

// Len implements sort.Interface.
func (s ImageInstanceSlice) Len() int {
	return len(s)
}

// Swap implements sort.Interface.
func (s ImageInstanceSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// ListInstances implements Client.
func (c *ClientImpl) ListInstances(ctx context.Context, repository string) (map[string]*ImageInstance, error) {
	url := fmt.Sprintf(listTagsURLTemplate, c.registry, repository, pageSize)
	rv := map[string]*ImageInstance{}
	if err := c.paginate(ctx, url, func(results paginatedResults) error {
		for digest, instance := range results.Manifest {
			inst, err := instance.imageInstance(digest)
			if err != nil {
				return err
			}
			rv[inst.Digest] = inst
		}
		return nil
	}); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// ListTags implements Client.
func (c *ClientImpl) ListTags(ctx context.Context, repository string) ([]string, error) {
	url := fmt.Sprintf(listTagsURLTemplate, c.registry, repository, pageSize)
	var rv []string
	if err := c.paginate(ctx, url, func(results paginatedResults) error {
		rv = append(rv, results.Tags...)
		return nil
	}); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// ImageConfig is the configuration blob for a Docker image instance.
type ImageConfig struct {
	Architecture  string                `json:"architecture"`
	Author        string                `json:"author"`
	Config        ImageConfig_Config    `json:"config"`
	Container     string                `json:"container"`
	Created       time.Time             `json:"created"`
	DockerVersion string                `json:"docker_version"`
	History       []ImageConfig_History `json:"history"`
	OS            string                `json:"os"`
	RootFS        ImageConfig_RootFS    `json:"rootfs"`
}

type ImageConfig_History struct {
	Author     string    `json:"author"`
	Created    time.Time `json:"created"`
	CreatedBy  string    `json:"created_by"`
	EmptyLayer bool      `json:"empty_layer"`
}

type ImageConfig_Config struct {
	AttachStderr bool     `json:"AttachStderr"`
	AttachStdout bool     `json:"AttachStdout"`
	Cmd          []string `json:"Cmd"`
	Entrypoint   []string `json:"Entrypoint"`
	Env          []string `json:"Env"`
	Hostname     string   `json:"Hostname"`
	Image        string   `json:"Image"`
	User         string   `json:"User"`
}

type ImageConfig_RootFS struct {
	DiffIDs []string `json:"diff_ids"`
	Type    string   `json:"type"`
}

// GetConfig implements Client.
func (c *ClientImpl) GetConfig(ctx context.Context, repository, configDigest string) (*ImageConfig, error) {
	url := fmt.Sprintf(blobURLTemplate, c.registry, repository, configDigest)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	rv := new(ImageConfig)
	if err := json.NewDecoder(resp.Body).Decode(&rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// Assert that ClientImpl implements Client.
var _ Client = &ClientImpl{}

// GetConfig retrieves an image config. It is a shortcut for Client.GetManifest
// and Client.GetConfig.
func GetConfig(ctx context.Context, c *ClientImpl, repository, reference string) (*ImageConfig, error) {
	manifest, err := c.GetManifest(ctx, repository, reference)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return c.GetConfig(ctx, repository, manifest.Config.Digest)
}

// GetDigest retrieves the digest for the given image from the registry. It is a
// shortcut for Client.GetManifest and Manifest.Digest.
//
// Note: This could instead be part of Client, and we could send a HEAD request
// to the manifests URL. This would be a little more efficient in that it would
// send less data over the network.
func GetDigest(ctx context.Context, c Client, repository, tag string) (string, error) {
	manifest, err := c.GetManifest(ctx, repository, tag)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return manifest.Digest, nil
}
