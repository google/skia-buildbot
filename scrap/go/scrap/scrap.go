// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const maxScrapSize = 128 * 1024

var (
	ErrInvalidScrapType = errors.New("Invalid scrap type.")
	ErrInvalidScrapName = errors.New("Invalid scrap name.")
	ErrInvalidLanguage  = errors.New("Invalid language.")
	ErrInvalidHash      = errors.New("Invalid SHA256 hash.")
	ErrInvalidScrapSize = errors.New("Scrap is too large.")
)

// SHA256 is a SHA 256 hash encoded in hex.
type SHA256 string

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

// AllTypes is a slice of supported Types.
var AllTypes = []Type{SVG, SKSL, Particle}

// ToType converts a string to a Type, returning UnknownType if it is not a
// valid Type.
func ToType(s string) Type {
	for _, t := range AllTypes {
		if string(t) == s {
			return t
		}
	}
	return UnknownType
}

// validateType returns true if t is a valid Type.
func validateType(t Type) error {
	if ToType(string(t)) == UnknownType {
		return skerr.Wrapf(ErrInvalidScrapType, "got: %q", t)
	}
	return nil
}

// Lang is a programming language a scrap can be embedded in.
type Lang string

const (
	// CPP is the C++ language.
	CPP Lang = "cpp"

	// JS is the Javascript language.
	JS Lang = "js"

	// UnknownLang is an unknown language.
	UnknownLang Lang = ""
)

// AllLangs is the list of all supported Langs.
var AllLangs = []Lang{CPP, JS}

// ToLang converts a string to a Lang, returning UnknownLang if it not a valid
// Lang.
func ToLang(s string) Lang {
	for _, l := range AllLangs {
		if string(l) == s {
			return l
		}
	}
	return UnknownLang
}

// validateLang returns true if l is a valid Lang.
func validateLang(l Lang) error {
	if ToLang(string(l)) == UnknownLang {
		return skerr.Wrapf(ErrInvalidLanguage, "got: %q", l)
	}
	return nil
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

// ChildShader is the scrap id of a single child shader along with the name that
// the uniform should have to access it.
type ChildShader struct {
	UniformName     string
	ScrapHashOrName string
}

// SKSLMetaData is metadata for SKSL scraps.
type SKSLMetaData struct {
	// Uniforms are all the inputs to the shader.
	Uniforms []float32

	// ImageURL is the URL of an image to load as an input shader.
	ImageURL string

	// Child shaders. A slice because order is important when mapping uniform
	// names in code to child shaders passed to makeShaderWithChildren.
	Children []ChildShader
}

// ParticlesMetaData is metadata for Particle scraps.
type ParticlesMetaData struct {
}

// ScrapBody is the body of scrap stored in GCS and transported by the API.
type ScrapBody struct {
	Type Type
	Body string

	// Type specific metadata:
	SVGMetaData       *SVGMetaData       `json:",omitempty"`
	SKSLMetaData      *SKSLMetaData      `json:",omitempty"`
	ParticlesMetaData *ParticlesMetaData `json:",omitempty"`
}

// ScrapID contains the identity of a newly created scrap.
type ScrapID struct {
	Hash SHA256
}

// Name has information about a single named scrap.
type Name struct {
	Hash        SHA256
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

// scrapExchange implements ScrapExchange.
type scrapExchange struct {
	client    gcs.GCSClient
	templates templateMap
}

// New returns a new instance of ScrapExchange.
func New(client gcs.GCSClient) (*scrapExchange, error) {
	tmpl, err := loadTemplates()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse templates")
	}

	return &scrapExchange{
		client:    client,
		templates: tmpl,
	}, nil
}

var validName = regexp.MustCompile("^@[0-9a-zA-Z-_]+$")

// validateName returns true if s is a valid name for a scrap.
func validateName(s string) error {
	if !validName.MatchString(s) {
		return skerr.Wrapf(ErrInvalidScrapName, "got: %q", s)
	}
	return nil
}

var validSHA256Hash = regexp.MustCompile("^[0-9a-f]{64}$")

// isValidHash returns true if s is a valid SHA256 hash.
func isValidHash(s SHA256) bool {
	return validSHA256Hash.MatchString(string(s))
}

// validate returns an error if the given scrap is invalid.
//
// validate may also modify the scrap to make it valid, so call this before
// calculating the hash for a scrap.
func (s *scrapExchange) validate(scrap *ScrapBody) error {
	if err := validateType(scrap.Type); err != nil {
		return err
	}
	return nil
}

// Expand the given scrap into a full program in the given language and write
// that code to the given io.Writer.
func (s *scrapExchange) Expand(ctx context.Context, t Type, hashOrName string, lang Lang, w io.Writer) error {
	if err := validateLang(lang); err != nil {
		return err
	}
	scrapBody, err := s.LoadScrap(ctx, t, hashOrName)
	if err != nil {
		return skerr.Wrapf(err, "Failed to load scrap.")
	}
	// TODO(jcgregorio) Add helpers to the template to allow recursively loading child scraps.
	err = s.templates[lang][t].Execute(w, scrapBody)
	if err != nil {
		return skerr.Wrapf(err, "Failed to expand template.")
	}

	return nil
}

// LoadScrap loads a scrap. The 'name' can be either a hash, or if prefixed with
// an "@" it is the name of scrap.
func (s *scrapExchange) LoadScrap(ctx context.Context, t Type, hashOrName string) (ScrapBody, error) {
	var ret ScrapBody

	if err := validateType(t); err != nil {
		return ret, err
	}

	var hash SHA256
	if err := validateName(hashOrName); err == nil {
		name := hashOrName
		nameBody, err := s.GetName(ctx, t, name)
		if err != nil {
			return ret, skerr.Wrapf(err, "Failed to get hash of name to load.")
		}
		hash = nameBody.Hash
	} else {
		hash = SHA256(hashOrName)
	}
	if !isValidHash(hash) {
		return ret, skerr.Wrap(ErrInvalidHash)
	}

	rc, err := s.client.FileReader(ctx, fmt.Sprintf("scraps/%s/%s", t, hash))
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to open scrap.")
	}
	defer util.Close(rc)
	if err := json.NewDecoder(rc).Decode(&ret); err != nil {
		return ret, skerr.Wrapf(err, "Failed to decode scrap.")
	}
	return ret, nil
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
	if b.Len() > maxScrapSize {
		return ret, ErrInvalidScrapSize
	}
	unencodedBody := b.Bytes()
	hashAsByteArray := sha256.Sum256(unencodedBody)
	hash := hex.EncodeToString(hashAsByteArray[:])
	ret.Hash = SHA256(hash)
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
	if err := validateType(t); err != nil {
		return err
	}

	var hash SHA256
	if err := validateName(hashOrName); err == nil {
		name := hashOrName
		nameBody, err := s.GetName(ctx, t, name)
		if err != nil {
			return skerr.Wrapf(err, "Failed to get hash of name to delete.")
		}
		err = s.DeleteName(ctx, t, name)
		if err != nil {
			return skerr.Wrapf(err, "Failed to delete name.")
		}
		hash = nameBody.Hash
	} else {
		hash = SHA256(hashOrName)
	}

	if !isValidHash(hash) {
		return skerr.Wrap(ErrInvalidHash)
	}

	err := s.client.DeleteFile(ctx, fmt.Sprintf("scraps/%s/%s", t, hash))
	if err != nil {
		return skerr.Wrapf(err, "Failed to delete hash.")
	}
	return nil
}

// PutName creates or updates a name for a given scrap.
func (s *scrapExchange) PutName(ctx context.Context, t Type, name string, nameBody Name) error {
	if err := validateType(t); err != nil {
		return err
	}
	if err := validateName(name); err != nil {
		return err
	}
	if !isValidHash(nameBody.Hash) {
		return skerr.Wrap(ErrInvalidHash)
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

	if err := validateType(t); err != nil {
		return ret, err
	}
	if err := validateName(name); err != nil {
		return ret, err
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
	if err := validateType(t); err != nil {
		return err
	}
	if err := validateName(name); err != nil {
		return err
	}

	err := s.client.DeleteFile(ctx, fmt.Sprintf("names/%s/%s", t, name))
	if err != nil {
		return skerr.Wrapf(err, "Failed to delete name.")
	}
	return nil
}

// ListNames lists all the known names for a given type of scrap.
func (s *scrapExchange) ListNames(ctx context.Context, t Type) ([]string, error) {
	ret := []string{}

	if err := validateType(t); err != nil {
		return nil, err
	}

	err := s.client.AllFilesInDirectory(ctx, fmt.Sprintf("names/%s/", t), func(item *storage.ObjectAttrs) {
		// GCS always uses forward slashes.
		name := path.Base(item.Name)
		ret = append(ret, name)
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read directory.")
	}

	return ret, nil
}

// Confirm that scrapExchange implements the ScrapExchange interface.
var _ ScrapExchange = (*scrapExchange)(nil)
