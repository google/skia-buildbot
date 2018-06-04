// Keeps a cache of all the source image thumbnails.
package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Source handles the source images that may be used by fiddles.
type Source struct {
	// thumbnails maps source image ids to the PNG bytes of the thumbnail.
	thumbnails map[int][]byte
}

// New create a new Source.
func New(dir string) (*Source, error) {
	s := &Source{
		thumbnails: map[int][]byte{},
	}
	filenames, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		return nil, fmt.Errorf("Failed loading sources: %s", err)
	}
	for _, filename := range filenames {
		err := util.WithReadFile(filename, func(f io.Reader) error {
			img, err := png.Decode(f)
			if err != nil {
				return err
			}
			img = resize.Resize(64, 64, img, resize.NearestNeighbor)
			buf := &bytes.Buffer{}
			if err := png.Encode(buf, img); err != nil {
				return fmt.Errorf("Failed to encode thumbnail: %s", err)
			}
			i, err := strconv.Atoi(strings.Split(filepath.Base(filename), ".")[0])
			if err != nil {
				return fmt.Errorf("Source filename isn't an integer: %s", err)
			}
			s.thumbnails[i] = buf.Bytes()
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("Failed to download source image: %s", err)
		}
	}
	return s, err
}

// List returns the list of source image ids.
func (s *Source) List() []int {
	ret := []int{}
	for i := range s.thumbnails {
		ret = append(ret, i)
	}
	sort.Sort(sort.IntSlice(ret))
	return ret
}

// ListAsJSON return s.List() serialized as JSON.
func (s *Source) ListAsJSON() string {
	b, err := json.Marshal(s.List())
	if err != nil {
		sklog.Errorf("Failed to encode as JSON: %s", err)
		return ""
	}
	return string(b)
}

// Thumbnail returns the serialized PNG thumbnail for the given source id.
func (s *Source) Thumbnail(i int) ([]byte, bool) {
	b, ok := s.thumbnails[i]
	return b, ok
}
