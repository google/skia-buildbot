package types

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
