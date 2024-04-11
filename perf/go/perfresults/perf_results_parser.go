package perfresults

import (
	"encoding/json"
	"io"
	"math"
	"slices"

	"go.skia.org/infra/go/skerr"
)

// PerfResults represents the contenst of a perf_results.json file generated by a
// telemetry-based benchmark. The full format is not formally defined, but some
// documnentation for it exists in various places.  The most comprehensive doc is
// https://chromium.googlesource.com/external/github.com/catapult-project/catapult/+/HEAD/docs/Histogram-set-json-format.md
type PerfResults struct {
	Histograms map[string]Histogram
}

// NonEmptyHistogramNames returns a list of names of histograms whose SampleValues arrays are non-empty.
func (pr *PerfResults) NonEmptyHistogramNames() []string {
	ret := []string{}
	for _, h := range pr.Histograms {
		if len(h.SampleValues) > 0 {
			ret = append(ret, h.Name)
		}
	}
	return ret
}

// Histogram is an individual benchmark measurement.
type Histogram struct {
	Name string `json:"name"`
	Unit string `json:"unit"`

	// optional fields
	Description  string    `json:"description"`
	SampleValues []float64 `json:"sampleValues"`
	// Diagnostics maps a diagnostic key to a guid, which points to e.g. a genericSet.
	Diagnostics map[string]any `json:"diagnostics"`
}

// GenericSet is a normalized value that other parts of the json file can reference by guid.
type GenericSet struct {
	Values []any `json:"values"` // Can be string or number. sigh.
}

// DateRange is a range of dates.
type DateRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// RelatedNameMap is a map from short names to full histogram names.
type RelatedNameMap struct {
	Names map[string]string `json:"names"`
}

type singleEntry struct {
	Type string `json:"type"`
	GUID string `json:"guid"`

	Histogram
	GenericSet
	DateRange
	RelatedNameMap
}

func (h *Histogram) populateDiagnostics(metadata map[string]any) {
	// no-op
	// We don't use the diagnostics now, it contains extra data and may consume more memories
	// than needed. We save this for later work if we need to surface any info here.
}

// NewResults creates a new PerfResults from the given data stream.
//
// It decodes the data in a streaming manner to reduce the memory footprint as the JSON files
// are sometimes bigger than 10MB.
func NewResults(r io.Reader) (*PerfResults, error) {
	pr := &PerfResults{
		Histograms: make(map[string]Histogram),
	}
	decoder := json.NewDecoder(r)

	// perf_results.json is an array of objects
	// read the open '['
	t, err := decoder.Token()

	// don't panic on an empty file
	if err == io.EOF {
		return pr, nil
	}
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if delim, ok := t.(json.Delim); !ok || delim.String() != "[" {
		return nil, skerr.Fmt("expecting the open '['")
	}

	// metadata only useful within the file scope.
	md := make(map[string]any)

	// looping all the elements
	for decoder.More() {
		var entry singleEntry
		err := decoder.Decode(&entry)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		// If Name is not empty, it is a histogram
		if entry.Name != "" {
			entry.populateDiagnostics(md)
			pr.Merge(entry.Histogram)
			continue
		}
		switch entry.Type {
		case "GenericSet":
			md[entry.GUID] = entry.GenericSet
		case "DateRange":
			md[entry.GUID] = entry.DateRange
		case "RelatedNameMap":
			md[entry.GUID] = entry.RelatedNameMap
		}
	}

	t, err = decoder.Token()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if delim, ok := t.(json.Delim); !ok || delim.String() != "]" {
		return nil, skerr.Fmt("expecting the closing ']'")
	}

	return pr, nil
}

// This should be deprecated in favor of streaming decoding.
//
// UnmarshalJSON parses a byte slice into a PerfResults instance.
func (pr *PerfResults) UnmarshalJSON(data []byte) error {
	pr.Histograms = make(map[string]Histogram)
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	md := make(map[string]any)
	for _, m := range raw {
		var entry singleEntry
		if err := json.Unmarshal(m, &entry); err != nil {
			return err
		}
		// If Name is not empty, it is a histogram
		if entry.Name != "" {
			entry.populateDiagnostics(md)
			pr.Merge(entry.Histogram)
			continue
		}
		switch entry.Type {
		case "GenericSet":
			md[entry.GUID] = entry.GenericSet
		case "DateRange":
			md[entry.GUID] = entry.DateRange
		case "RelatedNameMap":
			md[entry.GUID] = entry.RelatedNameMap
		}
	}
	return nil
}

func (pr *PerfResults) GetSampleValues(chart string) []float64 {
	if h, ok := pr.Histograms[chart]; ok {
		return h.SampleValues
	} else {
		return nil
	}
}

// Merge takes the given histogram and merges sample values.
func (pr *PerfResults) Merge(other Histogram) {
	if h, ok := pr.Histograms[other.Name]; ok {
		other.SampleValues = append(h.SampleValues, other.SampleValues...)
	}
	pr.Histograms[other.Name] = other
}

func (h Histogram) Min() float64 {
	return slices.Min(h.SampleValues)
}

func (h Histogram) Max() float64 {
	return slices.Max(h.SampleValues)
}

func (h Histogram) Count() int {
	return len(h.SampleValues)
}

func (h Histogram) Mean() float64 {
	return h.Sum() / float64(h.Count())
}

func (h Histogram) Stddev() float64 {
	sum := h.Sum()
	mean := sum / float64(h.Count())
	vr := 0.0
	for _, x := range h.SampleValues {
		vr += (x - mean) * (x - mean)
	}
	stddev := math.Sqrt(float64(vr / float64(h.Count()-1)))
	return stddev
}

func (h Histogram) Sum() float64 {
	s := 0.0
	for i := range h.SampleValues {
		s += h.SampleValues[i]
	}
	return s
}