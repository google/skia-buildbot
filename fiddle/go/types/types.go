package types

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strings"

	"go.skia.org/infra/fiddle/go/linenumbers"
)

// Result is the JSON output format from fiddle_run.
type Result struct {
	Errors  string  `json:"errors"`
	Compile Compile `json:"compile"`
	Execute Execute `json:"execute"`
}

// Compile contains the out from compiling the user's fiddle.
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
	Raster string `json:"Raster"`
	Gpu    string `json:"Gpu"`
	Pdf    string `json:"Pdf"`
	Skp    string `json:"Skp"`
}

// Options are the users options they can select when running a fiddle that
// will cause it to produce different output.
//
// If new fields are added make sure to update ComputeHash.
type Options struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Source int `json:"source"`
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
