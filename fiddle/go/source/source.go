// Keeps a cache of all the source image thumbnails, updating the cache when
// new images are added.
package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"sort"

	"github.com/nfnt/resize"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/store"
)

// Source handles the source images that may be used by fiddles.
type Source struct {
	// thumbnails maps source image ids to the PNG bytes of the thumbnail.
	thumbnails map[int][]byte
	st         *store.Store
}

// New create a new Source.
func New(st *store.Store) (*Source, error) {
	s := &Source{
		thumbnails: map[int][]byte{},
		st:         st,
	}
	// Populate the cache.
	ids, err := st.ListSourceImages()
	if err != nil {
		return nil, fmt.Errorf("Failed to list source images: %s", err)
	}
	for _, i := range ids {
		img, err := st.GetSourceImage(i)
		if err != nil {
			return nil, fmt.Errorf("Failed to download source image: %s", err)
		}
		img = resize.Resize(64, 64, img, resize.NearestNeighbor)
		buf := &bytes.Buffer{}
		if err := png.Encode(buf, img); err != nil {
			return nil, fmt.Errorf("Failed to encode thumbnail: %s", err)
		}
		s.thumbnails[i] = buf.Bytes()
	}
	return s, err
}

// List returns the list of source image ids.
func (s *Source) List() []int {
	ret := []int{}
	for i, _ := range s.thumbnails {
		ret = append(ret, i)
	}
	sort.Sort(sort.IntSlice(ret))
	return ret
}

// ListAsJSON return s.List() serialized as JSON.
func (s *Source) ListAsJSON() string {
	b, err := json.Marshal(s.List())
	if err != nil {
		glog.Errorf("Failed to encode as JSON: %s", err)
		return ""
	}
	return string(b)
}

// Thumbnail returns the serialized PNG thumbnail for the given source id.
func (s *Source) Thumbnail(i int) ([]byte, bool) {
	b, ok := s.thumbnails[i]
	return b, ok
}
