package binaryauthorization

/*
Package binaryauthorization provides an API client used for working with Binary
Authorization, in particular checking attestations for an image.

TODO(borenet): Remove this once a real API client exists.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	Scope = auth.ScopeAllCloudAPIs
)

type Attestor struct {
	Name                 string `json:"name"`
	UserOwnedGrafeasNote struct {
		NoteReference string `json:"noteReference"`
		PublicKeys    []struct {
			Id            string `json:"id"`
			PkixPublicKey struct {
				PublicKeyPem       string `json:"publicKeyPem"`
				SignatureAlgorithm string `json:"signatureAlgorithm"`
			} `json:"pkixPublicKey"`
		} `json:"publicKeys"`
	} `json:"userOwnedGrafeasNote"`
}

type Attestation struct {
	// TODO(borenet): I don't know what this looks like because I don't have
	// permission to access it.
}

type Client interface {
	GetAttestor(ctx context.Context, project, attestor string) (*Attestor, error)
	ListAttestations(ctx context.Context, project, attestor, image string) ([]*Attestation, error)
}

type ApiClient http.Client

func (c *ApiClient) GetAttestor(ctx context.Context, project, attestor string) (*Attestor, error) {
	url := fmt.Sprintf("https://binaryauthorization.googleapis.com/v1/projects/%s/attestors/%s?alt=json", project, attestor)
	resp, err := (*http.Client)(c).Get(url)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	var rv Attestor
	if err := json.NewDecoder(resp.Body).Decode(&rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &rv, nil
}

func (c *ApiClient) ListAttestations(ctx context.Context, project, attestor, sha256 string) ([]*Attestation, error) {
	a, err := c.GetAttestor(ctx, project, attestor)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	noteReference := path.Base(a.UserOwnedGrafeasNote.NoteReference)
	url := fmt.Sprintf("https://containeranalysis.googleapis.com/v1/projects/%s/notes/%s/occurrences?alt=json&filter=has_suffix%%28resourceUrl%%2C+%%22%s%%22%%29&pageSize=100", project, noteReference, sha256)
	//url := fmt.Sprintf("https://containeranalysis.googleapis.com/v1/projects/%s/notes/%s/occurrences?alt=json&pageSize=100", project, noteReference)
	resp, err := (*http.Client)(c).Get(url)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	fmt.Println(string(b))
	var rv []*Attestation
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

var _ Client = &ApiClient{}
