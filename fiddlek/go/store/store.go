// Package store stores and retrieves fiddles and associated assets in Google Storage.
package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	FIDDLE_STORAGE_BUCKET = "skia-fiddle"

	LRU_CACHE_SIZE = 10000

	// *_METADATA are the keys used to store the metadata values in Google Storage.
	USER_METADATA                   = "user"
	HASH_METADATA                   = "hash"
	STATUS_METADATA                 = "status"
	WIDTH_METADATA                  = "width"
	HEIGHT_METADATA                 = "height"
	SOURCE_METADATA                 = "source"
	SOURCE_MIPMAP_METADATA          = "source_mipmap"
	TEXTONLY_METADATA               = "textOnly"
	SRGB_METADATA                   = "srgb"
	F16_METADATA                    = "f16"
	ANIMATED_METADATA               = "animated"
	DURATION_METADATA               = "duration"
	OFFSCREEN_METADATA              = "offscreen"
	OFFSCREEN_WIDTH_METADATA        = "offscreen_width"
	OFFSCREEN_HEIGHT_METADATA       = "offscreen_height"
	OFFSCREEN_SAMPLE_COUNT_METADATA = "offscreen_sample_count"
	OFFSCREEN_TEXTURABLE_METADATA   = "offscreen_texturable"
	OFFSCREEN_MIPMAP_METADATA       = "offscreen_mipmap"
)

// Media is the type of outputs we can get from running a fiddle.
type Media string

// Media constants.
const (
	CPU      Media = "CPU"
	GPU      Media = "GPU"
	PDF      Media = "PDF"
	SKP      Media = "SKP"
	TXT      Media = "TXT"
	ANIM_CPU Media = "ANIM_CPU"
	ANIM_GPU Media = "ANIM_GPU"
	GLINFO   Media = "GLINFO"
	UNKNOWN  Media = ""
)

// props records the name and content-type for each type of Media and is used in mediaProps.
type props struct {
	filename    string
	contentType string
}

var (
	mediaProps = map[Media]props{
		CPU:      {filename: "cpu.png", contentType: "image/png"},
		GPU:      {filename: "gpu.png", contentType: "image/png"},
		PDF:      {filename: "pdf.pdf", contentType: "application/pdf"},
		SKP:      {filename: "skp.skp", contentType: "application/octet-stream"},
		TXT:      {filename: "txt.txt", contentType: "text/plain"},
		ANIM_CPU: {filename: "cpu.webm", contentType: "video/webm"},
		ANIM_GPU: {filename: "gpu.webm", contentType: "video/webm"},
		GLINFO:   {filename: "glinfo.text", contentType: "text/plain"},
	}

	// validName validates a fiddle name.
	validName = regexp.MustCompile("^[0-9a-zA-Z_]+$")
)

// cacheEntry is used to store PNGs in the Store lru cache.
type cacheEntry struct {
	body []byte
}

// Store is used to read and write user code and media to and from Google
// Storage.
type Store interface {
	// Put writes the code and media to Google Storage.
	//
	//    code - The user's code.
	//    options - The options the user chose to run the code under.
	//    results - The results from running fiddle_run.
	//
	// Code is written to:
	//
	//   gs://skia-fiddle/fiddle/<fiddleHash>/draw.cpp
	//
	// And media files are written to:
	//
	//   gs://skia-fiddle/fiddle/<fiddleHash>/cpu.png
	//   gs://skia-fiddle/fiddle/<fiddleHash>/gpu.png
	//   gs://skia-fiddle/fiddle/<fiddleHash>/skp.skp
	//   gs://skia-fiddle/fiddle/<fiddleHash>/pdf.pdf
	//
	// If results is nil then only the code is written.
	//
	// Returns the fiddleHash.
	Put(code string, options types.Options, results *types.Result) (string, error)

	// PutMedia writes the media for the given fiddleHash to Google Storage.
	//
	//    fiddleHash - The fiddle hash.
	//    results - The results from running fiddle_run.
	//
	// Media files are written to:
	//
	//   gs://skia-fiddle/fiddle/<fiddleHash>/cpu.png
	//   gs://skia-fiddle/fiddle/<fiddleHash>/gpu.png
	//   gs://skia-fiddle/fiddle/<fiddleHash>/skp.skp
	//   gs://skia-fiddle/fiddle/<fiddleHash>/pdf.pdf
	//
	// If results is nil then only the code is written.
	//
	// Returns the fiddleHash.
	PutMedia(options types.Options, fiddleHash string, results *types.Result) error

	// GetCode returns the code and options for the given fiddle hash.
	//
	//    fiddleHash - The fiddle hash.
	//
	// Returns the code and the options the code was run under.
	GetCode(fiddleHash string) (string, *types.Options, error)

	// GetMedia returns the file, content-type, filename, and error for a given fiddle hash and type of media.
	//
	//    fiddleHash - The hash of the fiddle.
	//    media - The type of the file to read.
	//
	// Returns the media file contents as a byte slice, the content-type, and the filename of the media.
	GetMedia(fiddleHash string, media Media) ([]byte, string, string, error)

	// ListAllNames returns the list of all named fiddles.
	ListAllNames() ([]Named, error)

	// GetHashFromName loads the fiddle hash for the given name.
	GetHashFromName(name string) (string, error)

	// ValidName returns true if the name conforms to the restrictions on names.
	//
	//   name - The name of the fidde.
	ValidName(name string) bool

	// WriteName writes the name file for a named fiddle.
	//
	//   name - The name of the fidde.
	//   hash - The fiddle hash.
	//   user - The email of the user that created the name.
	//   status - The current status of the named fiddle. An empty string means it
	//       is working. Non-empty string implies the fiddle is broken.
	WriteName(name, hash, user, status string) error

	// SetStatus updates just the status of a named fiddle.
	//
	//   name - The name of the fidde.
	//   status - The current status of the named fiddle. An empty string means it
	//       is working. Non-empty string implies the fiddle is broken.
	SetStatus(name, status string) error

	// DeleteName deletes a named fiddle.
	//
	//   name - The name of the fidde.
	DeleteName(name string) error

	// Exists returns true if the hash exists.
	//
	//   hash - A fiddle hash, maybe.
	Exists(hash string) error
}

// store implements Store.
type store struct {
	bucket *storage.BucketHandle

	// cache is an in-memory cache of PNGs, where the keys are <fiddlehash>-<media>.
	cache *lru.Cache
}

func cacheKey(fiddleHash string, media Media) string {
	return fiddleHash + "-" + string(media)
}

func shouldBeCached(media Media) bool {
	return media == CPU || media == GPU
}

// New creates a new *store, which implements Store.
//
// local - True if running locally.
func New(ctx context.Context, local bool) (*store, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeFullControl)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up client OAuth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Problem creating storage client: %s", err)
	}
	cache, err := lru.New(LRU_CACHE_SIZE)
	if err != nil {
		return nil, fmt.Errorf("Failed creating cache: %s", err)
	}
	return &store{
		bucket: storageClient.Bucket(FIDDLE_STORAGE_BUCKET),
		cache:  cache,
	}, nil
}

// writeMediaFile writes a file to Google Storage. It also adds it to the cache.
//
//    media - The type of the file to write.
//    fiddleHash - The hash of the fiddle.
//    b64 - The contents of the media file base64 encoded.
func (s *store) writeMediaFile(media Media, fiddleHash, b64 string) error {
	if b64 == "" && media != TXT {
		return fmt.Errorf("An empty file is not a valid %s file.", string(media))
	}
	p := mediaProps[media]
	if p.filename == "" {
		return fmt.Errorf("Unknown media type.")
	}
	body, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("Media wasn't properly encoded base64: %s", err)
	}

	// Only PNGs get stored in the cache.
	if shouldBeCached(media) {
		key := cacheKey(fiddleHash, media)
		sklog.Infof("Cache write: %s", key)
		if c, ok := s.cache.Get(key); !ok {
			s.cache.Add(key, &cacheEntry{
				body: body,
			})
		} else {
			if entry, ok := c.(*cacheEntry); ok {
				entry.body = body
			} else {
				sklog.Errorf("Found a non-cacheEntry in the lru Cache: %v", reflect.TypeOf(c))
			}
		}
	}

	// Don't stall the http response while we write the image to Google Storage.
	// Instead, do the work in a Go routine. We know that by the time we reach
	// here we've successfully written the code to Google Storage, so even if
	// this fails the user can always 'rerun' the fiddle to generate an image
	// that failed to write.
	go func() {
		path := strings.Join([]string{"fiddle", fiddleHash, p.filename}, "/")
		w := s.bucket.Object(path).NewWriter(context.Background())
		defer util.Close(w)
		w.ObjectAttrs.ContentEncoding = p.contentType
		if n, err := w.Write(body); err != nil {
			sklog.Errorf("There was a problem storing the media for %s. Uploaded %d bytes: %s", string(media), n, err)
		}
	}()
	return nil
}

// Put implements Store.
func (s *store) Put(code string, options types.Options, results *types.Result) (string, error) {
	fiddleHash, err := options.ComputeHash(code)
	if err != nil {
		return "", fmt.Errorf("Could not compute hash for the code: %s", err)
	}
	// Write code.
	path := strings.Join([]string{"fiddle", fiddleHash, "draw.cpp"}, "/")
	w := s.bucket.Object(path).NewWriter(context.Background())
	defer util.Close(w)
	w.ObjectAttrs.ContentEncoding = "text/plain"
	w.ObjectAttrs.Metadata = map[string]string{
		WIDTH_METADATA:                  fmt.Sprintf("%d", options.Width),
		HEIGHT_METADATA:                 fmt.Sprintf("%d", options.Height),
		SOURCE_METADATA:                 fmt.Sprintf("%d", options.Source),
		SOURCE_MIPMAP_METADATA:          fmt.Sprintf("%v", options.SourceMipMap),
		TEXTONLY_METADATA:               fmt.Sprintf("%v", options.TextOnly),
		SRGB_METADATA:                   fmt.Sprintf("%v", options.SRGB),
		F16_METADATA:                    fmt.Sprintf("%v", options.F16),
		ANIMATED_METADATA:               fmt.Sprintf("%v", options.Animated),
		DURATION_METADATA:               fmt.Sprintf("%f", options.Duration),
		OFFSCREEN_METADATA:              fmt.Sprintf("%v", options.OffScreen),
		OFFSCREEN_WIDTH_METADATA:        fmt.Sprintf("%d", options.OffScreenWidth),
		OFFSCREEN_HEIGHT_METADATA:       fmt.Sprintf("%d", options.OffScreenHeight),
		OFFSCREEN_SAMPLE_COUNT_METADATA: fmt.Sprintf("%d", options.OffScreenSampleCount),
		OFFSCREEN_TEXTURABLE_METADATA:   fmt.Sprintf("%v", options.OffScreenTexturable),
		OFFSCREEN_MIPMAP_METADATA:       fmt.Sprintf("%v", options.OffScreenMipMap),
	}
	if n, err := w.Write([]byte(code)); err != nil {
		return "", fmt.Errorf("There was a problem storing the code. Uploaded %d bytes: %s", n, err)
	}
	// Write media, if any.
	if results == nil {
		return fiddleHash, nil
	}
	if err := s.PutMedia(options, fiddleHash, results); err != nil {
		return fiddleHash, err
	}
	return fiddleHash, nil
}

// PutMedia implements Store.
func (s *store) PutMedia(options types.Options, fiddleHash string, results *types.Result) error {
	// Write each of the media files.
	if options.TextOnly {
		err := s.writeMediaFile(TXT, fiddleHash, results.Execute.Output.Text)
		if err != nil {
			return err
		}
	} else {
		if options.Animated {
			err := s.writeMediaFile(ANIM_CPU, fiddleHash, results.Execute.Output.AnimatedRaster)
			if err != nil {
				return err
			}
			err = s.writeMediaFile(ANIM_GPU, fiddleHash, results.Execute.Output.AnimatedGpu)
			if err != nil {
				return err
			}
		} else {
			err := s.writeMediaFile(CPU, fiddleHash, results.Execute.Output.Raster)
			if err != nil {
				return err
			}
			err = s.writeMediaFile(GPU, fiddleHash, results.Execute.Output.Gpu)
			if err != nil {
				return err
			}
			err = s.writeMediaFile(PDF, fiddleHash, results.Execute.Output.Pdf)
			if err != nil {
				return err
			}
			err = s.writeMediaFile(SKP, fiddleHash, results.Execute.Output.Skp)
			if err != nil {
				return err
			}
		}
	}
	if results.Execute.Output.GLInfo != "" {
		err := s.writeMediaFile(GLINFO, fiddleHash, results.Execute.Output.GLInfo)
		if err != nil {
			sklog.Warningf("Failed to save GLInfo: %s", err)
		}
	}
	return nil
}

// GetCode implements Store.
func (s *store) GetCode(fiddleHash string) (string, *types.Options, error) {
	o := s.bucket.Object(fmt.Sprintf("fiddle/%s/draw.cpp", fiddleHash))
	r, err := o.NewReader(context.Background())
	if err != nil {
		return "", nil, fmt.Errorf("Failed to open source file for %s: %s", fiddleHash, err)
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to read source file for %s: %s", fiddleHash, err)
	}
	attr, err := o.Attrs(context.Background())
	if err != nil {
		return "", nil, fmt.Errorf("Failed to read attributes for %s: %s", fiddleHash, err)
	}
	width, err := strconv.Atoi(attr.Metadata[WIDTH_METADATA])
	if err != nil {
		return "", nil, fmt.Errorf("Failed to parse options width: %s", err)
	}
	height, err := strconv.Atoi(attr.Metadata[HEIGHT_METADATA])
	if err != nil {
		return "", nil, fmt.Errorf("Failed to parse options height: %s", err)
	}
	source, err := strconv.Atoi(attr.Metadata[SOURCE_METADATA])
	if err != nil {
		return "", nil, fmt.Errorf("Failed to parse options source: %s", err)
	}
	animated := attr.Metadata[ANIMATED_METADATA] == "true"
	duration, err := strconv.ParseFloat(attr.Metadata[DURATION_METADATA], 64)
	if err != nil && animated {
		duration = 1.0
	}

	offscreen_width, err := strconv.Atoi(attr.Metadata[OFFSCREEN_WIDTH_METADATA])
	if err != nil {
		offscreen_width = 0
	}
	offscreen_height, err := strconv.Atoi(attr.Metadata[OFFSCREEN_HEIGHT_METADATA])
	if err != nil {
		offscreen_height = 0
	}
	offscreen_sample_count, err := strconv.Atoi(attr.Metadata[OFFSCREEN_SAMPLE_COUNT_METADATA])
	if err != nil {
		offscreen_sample_count = 0
	}
	options := &types.Options{
		Width:                width,
		Height:               height,
		Source:               source,
		SourceMipMap:         attr.Metadata[SOURCE_MIPMAP_METADATA] == "true",
		TextOnly:             attr.Metadata[TEXTONLY_METADATA] == "true",
		SRGB:                 attr.Metadata[SRGB_METADATA] == "true",
		F16:                  attr.Metadata[F16_METADATA] == "true",
		Animated:             animated,
		Duration:             duration,
		OffScreen:            attr.Metadata[OFFSCREEN_METADATA] == "true",
		OffScreenWidth:       offscreen_width,
		OffScreenHeight:      offscreen_height,
		OffScreenSampleCount: offscreen_sample_count,
		OffScreenTexturable:  attr.Metadata[OFFSCREEN_TEXTURABLE_METADATA] == "true",
		OffScreenMipMap:      attr.Metadata[OFFSCREEN_MIPMAP_METADATA] == "true",
	}
	return string(b), options, nil
}

// GetMedia implements Store.
func (s *store) GetMedia(fiddleHash string, media Media) ([]byte, string, string, error) {
	ctx := context.Background()
	key := cacheKey(fiddleHash, media)
	if c, ok := s.cache.Get(key); ok {
		if entry, ok := c.(*cacheEntry); ok {
			sklog.Infof("Cache hit: %s", key)
			return entry.body, mediaProps[media].contentType, mediaProps[media].filename, nil
		}
	}

	prefix := fmt.Sprintf("fiddle/%s/", fiddleHash)
	r, err := s.bucket.Object(prefix + mediaProps[media].filename).NewReader(ctx)
	if err != nil {
		// Legacy support for how images used to be stored.
		//
		// Fiddle results used to be stored per 'run' which included the githash and timestamp
		// of the githash.
		//
		// List the dirs under gs://skia-fiddle/fiddle/<fiddleHash>/ and find the most recent one.
		// Use Delimiter and Prefix to get a directory listing of sub-directories. See
		// https://cloud.google.com/storage/docs/json_api/v1/objects/list
		q := &storage.Query{
			Delimiter: "/",
			Prefix:    prefix,
		}
		runIds := []string{}
		it := s.bucket.Objects(ctx, q)
		for obj, err := it.Next(); err != iterator.Done; obj, err = it.Next() {
			if err != nil {
				return nil, "", "", fmt.Errorf("Failed to retrieve list of results for (%s, %s): %s", fiddleHash, string(media), err)
			}
			if obj.Prefix != "" {
				runIds = append(runIds, obj.Prefix)
			}
		}
		if len(runIds) == 0 {
			return nil, "", "", fmt.Errorf("This fiddle has no valid output written (%s, %s)", fiddleHash, string(media))
		}
		sort.Strings(runIds)
		r, err = s.bucket.Object(runIds[len(runIds)-1] + mediaProps[media].filename).NewReader(ctx)
		if err != nil {
			return nil, "", "", fmt.Errorf("Unable to get reader for the media file (%s, %s): %s", fiddleHash, string(media), err)
		}
	}
	defer util.Close(r)
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, "", "", fmt.Errorf("Unable to read the media file (%s, %s): %s", fiddleHash, string(media), err)
	}
	if shouldBeCached(media) {
		s.cache.Add(cacheKey(fiddleHash, media), &cacheEntry{
			body: b,
		})
	}
	return b, mediaProps[media].contentType, mediaProps[media].filename, nil
}

// Named is the information about a named fiddle.
type Named struct {
	Name   string
	User   string
	Hash   string
	Status string // If a non-empty string then this named fiddle is broken and the string contains some information about the breakage.
}

// ListAllNames implements Store.
func (s *store) ListAllNames() ([]Named, error) {
	ret := []Named{}
	ctx := context.Background()
	q := &storage.Query{
		Prefix: fmt.Sprintf("named/"),
	}
	it := s.bucket.Objects(ctx, q)
	for obj, err := it.Next(); err != iterator.Done; obj, err = it.Next() {
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve name list: %s", err)
		}
		filename := strings.Split(obj.Name, "/")[1]
		named := Named{
			Name:   filename,
			User:   obj.Metadata[USER_METADATA],
			Hash:   obj.Metadata[HASH_METADATA],
			Status: obj.Metadata[STATUS_METADATA],
		}
		if named.Hash == "" {
			sklog.Infof("Need to update metadata: %v", named)
			// Read the file contents and update the hash metadata.
			hash, err := s.GetHashFromName(named.Name)
			if err != nil {
				return nil, fmt.Errorf("Failed to read named hash in ListAllNames: %s", err)
			}
			named.Hash = hash
			if err := s.WriteName(named.Name, named.Hash, named.User, named.Status); err != nil {
				return nil, fmt.Errorf("Failed to update hash metadata: %s", err)
			}
		}
		ret = append(ret, named)
	}
	return ret, nil
}

// GetHashFromName implements Store.
func (s *store) GetHashFromName(name string) (string, error) {
	ctx := context.Background()
	r, err := s.bucket.Object(fmt.Sprintf("named/%s", name)).NewReader(ctx)
	if err != nil {
		return "", fmt.Errorf("Failed to open reader for name %q: %s", name, err)
	}
	defer util.Close(r)
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("Failed to read named file %q: %s", name, err)
	}
	return string(b), nil
}

// ValidName implements Store.
func (s *store) ValidName(name string) bool {
	return validName.MatchString(name)
}

// WriteName implements Store.
func (s *store) WriteName(name, hash, user, status string) error {
	if !s.ValidName(name) {
		return fmt.Errorf("Invalid character found in name.")
	}
	ctx := context.Background()
	w := s.bucket.Object(fmt.Sprintf("named/%s", name)).NewWriter(ctx)
	w.ObjectAttrs.Metadata = map[string]string{
		USER_METADATA:   user,
		HASH_METADATA:   hash,
		STATUS_METADATA: status,
	}
	if _, err := w.Write([]byte(hash)); err != nil {
		return fmt.Errorf("Failed to write named file %q: %s", name, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("Failed to close after writing named file %q: %s", name, err)
	}
	return nil
}

// SetStatus implements Store.
func (s *store) SetStatus(name, status string) error {
	if !s.ValidName(name) {
		return fmt.Errorf("Invalid character found in name.")
	}
	ctx := context.Background()
	atts := storage.ObjectAttrsToUpdate{
		Metadata: map[string]string{
			STATUS_METADATA: status,
		},
	}
	if _, err := s.bucket.Object(fmt.Sprintf("named/%s", name)).Update(ctx, atts); err != nil {
		return fmt.Errorf("Failed to update attributes for named file %q: %s", name, err)
	}
	return nil
}

// DeleteName implements Store.
func (s *store) DeleteName(name string) error {
	ctx := context.Background()
	return s.bucket.Object(fmt.Sprintf("named/%s", name)).Delete(ctx)
}

// Exists implements Store.
func (s *store) Exists(hash string) error {
	ctx := context.Background()
	o := s.bucket.Object(fmt.Sprintf("fiddle/%s/draw.cpp", hash))
	_, err := o.Attrs(ctx)
	return err
}

// Confirm the *store implements Store.
var _ Store = (*store)(nil)
