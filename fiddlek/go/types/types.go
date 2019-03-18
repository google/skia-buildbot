package types

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strings"

	"go.skia.org/infra/fiddlek/go/linenumbers"
)

const (
	// MAX_JSON_SIZE is the limit on the size of JSON produced by a single fiddle run.
	MAX_JSON_SIZE = 100 * 1024 * 1024

	MAX_CODE_SIZE = 128 * 1024
)

// Result is the JSON output format from fiddle_run.
type Result struct {
	Errors  string  `json:"errors"`
	Compile Compile `json:"compile"`
	Execute Execute `json:"execute"`
}

// Compile contains the output from compiling the user's fiddle.
type Compile struct {
	Errors string `json:"errors"`
	Output string `json:"output"` // Compiler output.
}

// Execute contains the output from running the compiled fiddle.
type Execute struct {
	Errors string `json:"errors"`
	Output Output `json:"output"`
}

// Output contains the base64 encoded files for each
// of the output types.
type Output struct {
	Raster         string `json:"Raster"`
	Gpu            string `json:"Gpu"`
	Pdf            string `json:"Pdf"`
	Skp            string `json:"Skp"`
	Text           string `json:"Text"`
	AnimatedRaster string `json:"AnimatedRaster"`
	AnimatedGpu    string `json:"AnimatedGpu"`
	GLInfo         string `json:"GLInfo"`
}

// Options are the users options they can select when running a fiddle that
// will cause it to produce different output.
//
// If new fields are added make sure to update ComputeHash and go/store.
type Options struct {
	Width                int     `json:"width"`
	Height               int     `json:"height"`
	Source               int     `json:"source"`
	SourceMipMap         bool    `json:"source_mipmap"`
	SRGB                 bool    `json:"srgb"`
	F16                  bool    `json:"f16"`
	TextOnly             bool    `json:"textOnly"`
	Animated             bool    `json:"animated"`
	Duration             float64 `json:"duration"`
	OffScreen            bool    `json:"offscreen"`
	OffScreenWidth       int     `json:"offscreen_width"`
	OffScreenHeight      int     `json:"offscreen_height"`
	OffScreenSampleCount int     `json:"offscreen_sample_count"`
	OffScreenTexturable  bool    `json:"offscreen_texturable"`
	OffScreenMipMap      bool    `json:"offscreen_mipmap"`
}

// ComputeHash calculates the fiddleHash for the given code and options.
//
// It might seem a little odd to have this as a member function of Options, but
// it's more likely to get updated if Options ever gets changed.
//
// The hash computation is a bit convoluted because it needs to be
// backward compatible with the original version of fiddle so URLs
// don't break.
func (o *Options) ComputeHash(code string) (string, error) {
	lines := strings.Split(linenumbers.LineNumbers(code), "\n")
	out := []string{
		"DECLARE_bool(portableFonts);",
		fmt.Sprintf("// WxH: %d, %d", o.Width, o.Height),
	}
	if o.SRGB || o.F16 {
		out = append(out, fmt.Sprintf("// SRGB: %v, %v", o.SRGB, o.F16))
	}
	if o.TextOnly {
		out = append(out, fmt.Sprintf("// TextOnly: %v", o.TextOnly))
	}
	if o.Animated {
		out = append(out, fmt.Sprintf("// Animated: %v", o.Animated))
	}
	if o.Duration != 0.0 {
		out = append(out, fmt.Sprintf("// Duration: %f", o.Duration))
	}
	if o.OffScreen {
		out = append(out, fmt.Sprintf("// Offscreen WxHxS: %d, %d, %d", o.OffScreenWidth, o.OffScreenHeight, o.OffScreenSampleCount))
		out = append(out, fmt.Sprintf("// Offscreen Texturable: %v", o.OffScreenTexturable))
		out = append(out, fmt.Sprintf("// Offscreen MipMap: %v", o.OffScreenMipMap))
	}
	if o.SourceMipMap {
		out = append(out, fmt.Sprintf("// Source MipMap: %v", o.SourceMipMap))
	}
	for _, line := range lines {
		if strings.Contains(line, "%:") {
			return "", fmt.Errorf("Unable to compile source.")
		}
		out = append(out, line)
	}
	h := md5.New()
	if _, err := h.Write([]byte(strings.Join(out, "\n"))); err != nil {
		return "", fmt.Errorf("Failed to write md5: %v", err)
	}
	if err := binary.Write(h, binary.LittleEndian, int64(o.Source)); err != nil {
		return "", fmt.Errorf("Failed to write md5: %v", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// FiddleContext is the structure we use for expanding the index.html template.
//
// It is also used (without the Hash) as the incoming JSON request to /_/run.
type FiddleContext struct {
	Version   string  `json:"version"`    // The version of Skia that fiddle is running.
	Sources   string  `json:"sources"`    // All the source image ids serialized as a JSON array.
	Hash      string  `json:"fiddlehash"` // Can be the fiddle hash or the fiddle name.
	Code      string  `json:"code"`
	Name      string  `json:"name"`      // In a request can be the name to create for this fiddle.
	Overwrite bool    `json:"overwrite"` // In a request, should a name be overwritten if it already exists.
	Fast      bool    `json:"fast"`      // Fast, don't compile and run if a fiddle with this hash has already been compiled and run.
	Options   Options `json:"options"`
}

// CompileError is a single line of compiler error output, along with the line
// and column that the error occurred at.
type CompileError struct {
	Text string `json:"text"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// RunResults is the results we serialize to JSON as the results from a run.
type RunResults struct {
	CompileErrors []CompileError `json:"compile_errors"`
	RunTimeError  string         `json:"runtime_error"`
	FiddleHash    string         `json:"fiddleHash"`
	Text          string         `json:"text"`
}

type BulkRequest map[string]*FiddleContext
type BulkResponse map[string]*RunResults

// State is the state of a fiddler.
type State string

const (
	IDLE      State = "idle"
	WRITING   State = "writing"
	COMPILING State = "compiling"
	RUNNING   State = "running"
	ERROR     State = "error"
)

var (
	AllStates = []State{IDLE, WRITING, COMPILING, RUNNING, ERROR}
)

// FiddlerMainResponse is the JSON that fiddler responds with for a request to "/".
type FiddlerMainResponse struct {
	State   State  `json:"state"`
	Version string `json:"version"` // Skia Git Hash
}
