package tsuite

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

const (
	ImgExtension     = ".png"
	MetaDataFileName = "meta.json"
)

type Classifier interface {
	Classify(digest string, img *image.NRGBA) (float32, *image.NRGBA)
}

type CompatTestSuite struct {
	Tests []string `json:"tests"`

	classifiers map[string]Classifier
}

func New() *CompatTestSuite {
	return &CompatTestSuite{
		Tests:       []string{},
		classifiers: map[string]Classifier{},
	}
}

func (c *CompatTestSuite) Add(testName string, classifier Classifier) {
	c.Tests = append(c.Tests, testName)
	c.classifiers[testName] = classifier
}

func (c *CompatTestSuite) Save(w io.Writer) error {
	return c.saveToWriter(w)
}

func (c *CompatTestSuite) SaveToFile(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer util.Close(f)
	return c.saveToWriter(f)
}

func (c *CompatTestSuite) saveToWriter(w io.Writer) error {
	zipW := zip.NewWriter(w)

	for _, testName := range c.Tests {
		classifier, ok := c.classifiers[testName]
		if !ok {
			return fmt.Errorf("Unknown classifier: %s", testName)
		}

		// TODO(stephana): Generalize this in the classifier interface.
		m := classifier.(*Memorizer)
		if err := m.Save(testName, zipW); err != nil {
			return err
		}
	}

	if err := writeJsonToZip(zipW, MetaDataFileName, c); err != nil {
		return err
	}

	return zipW.Close()
}

func (c *CompatTestSuite) Evaluate(testName, digest string, img *image.NRGBA) (float32, error) {
	classifier, ok := c.classifiers[testName]
	if !ok {
		return 0.0, fmt.Errorf("Unknown test name: %s", testName)
	}
	posProb, _ := classifier.Classify(digest, img)
	return posProb, nil
}

// TODO(stephana): Remove !!!
func (c *CompatTestSuite) GetClassifiers() map[string]Classifier {
	return c.classifiers
}

func (c *CompatTestSuite) TestNames() []string {
	return c.Tests
}

func Load(r io.ReaderAt, size int64) (*CompatTestSuite, error) {
	zipR, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	return loadFromReader(zipR)
}

func LoadFromFile(filePath string) (*CompatTestSuite, error) {
	zipR, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, err
	}
	defer util.Close(zipR)
	return loadFromReader(&zipR.Reader)
}

func loadFromReader(zipR *zip.Reader) (*CompatTestSuite, error) {
	ret := &CompatTestSuite{}
	if err := readJsonFromZip(zipR, MetaDataFileName, ret); err != nil {
		return nil, err
	}
	ret.classifiers = map[string]Classifier{}

	for _, testName := range ret.Tests {
		memorizer, err := LoadMemorizer(testName, zipR)
		if err != nil {
			return nil, err
		}
		ret.classifiers[testName] = memorizer
	}
	return ret, nil
}

type Memorizer struct {
	// images used for classification.
	images map[string]*image.NRGBA

	// Classification is the label for each image.
	Classification map[string]types.Label
}

func NewMemorizer() *Memorizer {
	return &Memorizer{
		images:         map[string]*image.NRGBA{},
		Classification: map[string]types.Label{},
	}
}

func (m *Memorizer) GetImages() map[string]*image.NRGBA {
	return m.images
}

func (m *Memorizer) Add(digest string, img *image.NRGBA, label types.Label) {
	m.images[digest] = img
	m.Classification[digest] = label
}

func (m *Memorizer) Save(prefix string, w *zip.Writer) error {
	// Iterate over the images.
	for digest, img := range m.images {
		if err := writePNGToZip(w, filepath.Join(prefix, digest+ImgExtension), img); err != nil {
			return err
		}
	}
	return writeJsonToZip(w, filepath.Join(prefix, MetaDataFileName), m)
}

func LoadMemorizer(prefix string, r *zip.Reader) (*Memorizer, error) {
	ret := &Memorizer{}
	if err := readJsonFromZip(r, filepath.Join(prefix, MetaDataFileName), ret); err != nil {
		return nil, err
	}
	ret.images = map[string]*image.NRGBA{}

	for digest := range ret.Classification {
		img, err := readPNGFromZip(r, filepath.Join(prefix, digest+ImgExtension))
		if err != nil {
			return nil, err
		}
		ret.images[digest] = img
	}

	return ret, nil
}

func (m *Memorizer) Classify(digest string, img *image.NRGBA) (float32, *image.NRGBA) {
	if digest != "" {
		if _, ok := m.images[digest]; ok {
			return 1.0, nil
		}
	}

	var closestMinDiff = float32(math.Inf(1))
	var closestLabel types.Label
	var closestDiffImg *image.NRGBA
	for digest, refImage := range m.images {
		genDiffRec, diffImg := diff.DefaultDiffFn(refImage, img)
		diffRec := genDiffRec.(*diff.DiffMetrics)
		if !diffRec.DimDiffer && (diffRec.PixelDiffPercent < closestMinDiff) {
			closestMinDiff = diffRec.PixelDiffPercent
			closestLabel = m.Classification[digest]
			closestDiffImg = diffImg
		}
	}

	if closestLabel == types.NEGATIVE {
		return 0, closestDiffImg
	}

	return closestMinDiff, closestDiffImg
}

type AlwaysTrueClassifier struct{}

func (a AlwaysTrueClassifier) Classify(digest string, img *image.NRGBA) (float32, *image.NRGBA) {
	return 1.0, nil
}

func writePNGToZip(w *zip.Writer, filePath string, img *image.NRGBA) error {
	return writeFileToZip(w, filePath, func(writer io.Writer) error {
		return diff.WritePNG(writer, img)
	})
}

func writeJsonToZip(w *zip.Writer, filePath string, src interface{}) error {
	return writeFileToZip(w, filePath, func(writer io.Writer) error {
		return json.NewEncoder(writer).Encode(src)
	})
}

func writeFileToZip(w *zip.Writer, filePath string, writeFn func(io.Writer) error) error {
	outF, err := w.Create(filePath)
	if err != nil {
		return err
	}

	return writeFn(outF)
}

func readPNGFromZip(r *zip.Reader, filePath string) (*image.NRGBA, error) {
	var ret *image.NRGBA = nil
	err := readFileFromZip(r, filePath, func(r io.Reader) error {
		var err error = nil
		ret, err = diff.OpenNRGBA(r)
		return err
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func readJsonFromZip(r *zip.Reader, filePath string, target interface{}) error {
	return readFileFromZip(r, filePath, func(r io.Reader) error {
		if err := json.NewDecoder(r).Decode(target); err != nil {
			return err
		}
		return nil
	})
}

func readFileFromZip(r *zip.Reader, filePath string, fn func(io.Reader) error) error {
	for _, fHeader := range r.File {
		if fHeader.Name == filePath {
			f, err := fHeader.Open()
			if err != nil {
				return err
			}
			defer util.Close(f)

			return fn(f)
		}
	}
	return fmt.Errorf("Could not find %s in zip file.", filePath)
}
