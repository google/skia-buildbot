// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

var (
	errInvalidScrapType = errors.New("Invalid scrap type.")
	errInvalidScrapName = errors.New("Invalid scrap name.")
	errInvalidHash      = errors.New("Invalid SHA256 hash.")
)

// Type identifies the type of a scrap.
type Type string

const (
	// SVG scrap.
	SVG Type = "svg"

	// SKSL scrap.
	SKSL Type = "sksl"

	// Particle scrap.
	Particle Type = "particle"

	// UnknownType type of scrap.
	UnknownType Type = ""
)

var allTypes = []Type{SVG, SKSL, Particle}

// ToType converts a string to a Type, returning UnknownType if it is not a
// valid Type.
func ToType(s string) Type {
	for _, t := range allTypes {
		if string(t) == s {
			return t
		}
	}
	return UnknownType
}

func isValidType(t Type) bool {
	return ToType(string(t)) != UnknownType
}

// Lang a programming language a scrap can be embedded in.
type Lang string

const (
	// CPP is the C++ language.
	CPP Lang = "cpp"

	// JS is the Javascript language.
	JS Lang = "js"

	// UnknownLang is an unknown language.
	UnknownLang Lang = ""
)

var allLangs = []Lang{CPP, JS}

// ToLang converts a string to a Lang, returning UnknownLang if it not a valid
// Lang.
func ToLang(s string) Lang {
	for _, l := range allLangs {
		if string(l) == s {
			return l
		}
	}
	return UnknownLang
}

// MimeTypes for each scrap type when served raw.
var MimeTypes = map[Type]string{
	SVG:      "image/svg+xml",
	SKSL:     "text/plain",
	Particle: "application/json",
}

// SVGMetaData is metadata for SVG scraps.
type SVGMetaData struct {
}

// Uniform is a single uniform for an SkSL shader.
type Uniform struct {
	Name  string
	Value float64
}

// SKSLMetaData is metadata for SKSL scraps.
type SKSLMetaData struct {
	// Uniforms are all the inputs to the shader.
	Uniforms []Uniform

	// Child shaders. These values are the hashes of shaders, or, if the value
	// begins with an "@", they are the name of a named shader.
	Children []string
}

// ParticlesMetaData is metadata for Particle scraps.
type ParticlesMetaData struct {
}

// ScrapBody is the body of scrap stored in GCS and transported by the API.
type ScrapBody struct {
	Type Type
	Body string

	// Type specific metadata:
	SVGMetaData       SVGMetaData       `json:",omitempty"`
	SKSLMetaData      SKSLMetaData      `json:",omitempty"`
	ParticlesMetaData ParticlesMetaData `json:",omitempty"`
}

// ScrapID contains the identity of a newly created scrap.
type ScrapID struct {
	Hash string
}

// Name has information about a single named scrap.
type Name struct {
	Hash        string
	Description string
}

// ScrapExchange handles reading and writing scraps.
type ScrapExchange interface {
	// Expand the given scrap into a full program in the given language and write
	// that code to the given io.Writer.
	Expand(ctx context.Context, t Type, hashOrName string, lang Lang, w io.Writer) error

	// LoadScrap loads a scrap. The 'name' can be either a hash, or if prefixed with
	// an "@" it is the name of scrap.
	LoadScrap(ctx context.Context, t Type, hashOrName string) (ScrapBody, error)

	// CreateScrap and return the hash by the ScrapID.
	CreateScrap(ctx context.Context, scrap ScrapBody) (ScrapID, error)

	// DeleteScrap and also delete the name if hashOrName is a name, which is indicated by
	// the prefix "@".
	DeleteScrap(ctx context.Context, t Type, hashOrName string) error

	// PutName creates or updates a name for a given scrap.
	PutName(ctx context.Context, t Type, name string, nameBody Name) error

	// GetName retrieves the hash for the given named scrap.
	GetName(ctx context.Context, t Type, name string) (Name, error)

	// DeleteName removes the name for the given named scrap.
	DeleteName(ctx context.Context, t Type, name string) error

	// ListNames lists all the known names for a given type of scrap.
	ListNames(ctx context.Context, t Type) ([]string, error)
}

// scrapExchange handles reading and writing scraps.
type scrapExchange struct {
	client gcs.GCSClient
}

// New returns a new instance of ScrapExchange.
func New(client gcs.GCSClient) *scrapExchange {
	return &scrapExchange{
		client: client,
	}
}

var validName = regexp.MustCompile("^@[0-9a-zA-Z-_]+$")

func isValidName(s string) bool {
	return validName.MatchString(s)
}

var validSHA256Hash = regexp.MustCompile("^[0-9a-f]{64}$")

func isValidHash(s string) bool {
	return validSHA256Hash.MatchString(s)
}

// validate returns an error if the given scrap is invalid.
//
// validate may also modify the scrap to make it valid, so call this before
// calculating the hash for a scrap.
func (s *scrapExchange) validate(scrap *ScrapBody) error {
	if ToType(string(scrap.Type)) == UnknownType {
		return skerr.Wrap(errInvalidScrapType)
	}
	return nil
}

// Expand the given scrap into a full program in the given language and write
// that code to the given io.Writer.
func (s *scrapExchange) Expand(ctx context.Context, t Type, hashOrName string, lang Lang, w io.Writer) error {
	return skerr.Fmt("Not implemented")
}

// LoadScrap loads a scrap. The 'name' can be either a hash, or if prefixed with
// an "@" it is the name of scrap.
func (s *scrapExchange) LoadScrap(ctx context.Context, t Type, hashOrName string) (ScrapBody, error) {
	return ScrapBody{}, nil
}

// CreateScrap and return the hash by the ScrapID.
func (s *scrapExchange) CreateScrap(ctx context.Context, scrap ScrapBody) (ScrapID, error) {
	var ret ScrapID
	if err := s.validate(&scrap); err != nil {
		return ret, skerr.Wrapf(err, "Invalid scrap.")
	}
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(scrap); err != nil {
		return ret, skerr.Wrapf(err, "Failed to JSON encode scrap.")
	}
	unencodedBody := b.Bytes()
	hash := fmt.Sprintf("%x", sha256.Sum256(unencodedBody))
	ret.Hash = hash
	w := s.client.FileWriter(ctx, fmt.Sprintf("scraps/%s/%s", scrap.Type, hash), gcs.FileWriteOptions{
		ContentEncoding: "gzip",
		ContentType:     "application/json",
	})
	zw := gzip.NewWriter(w)
	_, err := zw.Write(unencodedBody)
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to write JSON body.")
	}
	if err := zw.Close(); err != nil {
		return ret, skerr.Wrapf(err, "Failed to close gzip writer.")
	}
	if err := w.Close(); err != nil {
		return ret, skerr.Wrapf(err, "Failed to close GCS Storage writer.")
	}
	return ret, nil
}

// DeleteScrap and also delete the name if hashOrName is a name, which is indicated by
// the prefix "@".
func (s *scrapExchange) DeleteScrap(ctx context.Context, t Type, hashOrName string) error {
	return nil
}

// PutName creates or updates a name for a given scrap.
func (s *scrapExchange) PutName(ctx context.Context, t Type, name string, nameBody Name) error {
	if !isValidType(t) {
		return skerr.Wrap(errInvalidScrapType)
	}
	if !isValidName(name) {
		return skerr.Wrap(errInvalidScrapName)
	}
	if !isValidHash(nameBody.Hash) {
		return skerr.Wrap(errInvalidHash)
	}
	w := s.client.FileWriter(ctx, fmt.Sprintf("names/%s/%s", t, name), gcs.FileWriteOptions{
		ContentType: "application/json",
	})
	if err := json.NewEncoder(w).Encode(nameBody); err != nil {
		return skerr.Wrapf(err, "Failed to encode JSON.")
	}
	if err := w.Close(); err != nil {
		return skerr.Wrapf(err, "Failed to close GCS Storage writer.")
	}

	return nil
}

// GetName retrieves the hash for the given named scrap.
func (s *scrapExchange) GetName(ctx context.Context, t Type, name string) (Name, error) {
	var ret Name

	if !isValidType(t) {
		return ret, skerr.Wrap(errInvalidScrapType)
	}
	if !isValidName(name) {
		return ret, skerr.Wrap(errInvalidScrapName)
	}

	rc, err := s.client.FileReader(ctx, fmt.Sprintf("names/%s/%s", t, name))
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to open name.")
	}
	defer util.Close(rc)
	if err := json.NewDecoder(rc).Decode(&ret); err != nil {
		return ret, skerr.Wrapf(err, "Failed to decode body.")
	}
	return ret, nil
}

// DeleteName removes the name for the given named scrap.
func (s *scrapExchange) DeleteName(ctx context.Context, t Type, name string) error {
	if !isValidType(t) {
		return skerr.Wrap(errInvalidScrapType)
	}
	if !isValidName(name) {
		return skerr.Wrap(errInvalidScrapName)
	}

	return s.client.DeleteFile(ctx, fmt.Sprintf("names/%s/%s", t, name))
}

// ListNames lists all the known names for a given type of scrap.
func (s *scrapExchange) ListNames(ctx context.Context, t Type) ([]string, error) {
	ret := []string{}

	s.client.AllFilesInDirectory(ctx, fmt.Sprintf("names/%s/", t), func(item *storage.ObjectAttrs) {
		name := filepath.Base(item.Name)
		ret = append(ret, name)
	})

	return ret, nil
}

// Confirm that scrapExchange implements the ScrapExchange interface.
var _ ScrapExchange = (*scrapExchange)(nil)
