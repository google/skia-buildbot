package tsuite

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"go/types"
	"image"
	"io"
)

type Classifier interface {
	Classify(digest string, img *image.NRGBA) float32
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
	zipW := zip.NewWriter(w)

	jsonWriter, err := zipW.Create("meta.json")
	if err != nil {
		return err
	}

	if err := json.NewEncoder(jsonWriter).Encode(c); err != nil {
		return nil
	}

	for _, testName := range c.Tests {

	}
	// Write this object as json.

	// Write the classifiers.

	//

	return nil
}

func (c *CompatTestSuite) Evaluate(testName, digest string, img *image.NRGBA) (float32, error) {
	classifier, ok := c.classifiers[testName]
	if !ok {
		return 0.0, fmt.Errorf("Unknown test name: %s", testName)
	}
	return classifier.Classify(digest, img), nil
}

func (c *CompatTestSuite) TestNames() []string {
	return c.Tests
}

func Load(r io.Reader) (*CompatTestSuite, error) {
	return nil, nil
}

type Memorizer struct {
	images []*image.NRGBA
}

func NewMemorizer() *Memorizer {
	return &Memorizer{
		images: []*image.NRGBA{},
	}
}

func (m *Memorizer) Add(digest string, img *image.NRGBA, label types.Label) {
	m.images = append(m.images, img)
}

func (m *Memorizer) Save(prefix string, w *zip.Writer) error {
	return nil
}

type AlwaysTrueClassifier struct{}

func (a AlwaysTrueClassifier) Classify(digest string, img *image.NRGBA) float32 {
	return 1.0
}
