package npy

import (
	"log"
	"os"

	"github.com/sbinet/npyio"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// NpyReader provides a struct to read npy files.
type NpyReader struct {
	// The path to the npy file.
	filePath string
}

// NewNpyReader returns a new instance of NpyReader.
func NewNpyReader(filepath string) *NpyReader {
	return &NpyReader{
		filePath: filepath,
	}
}

// ReadFloat32 reads the supplied npy file using the float32 dtype.
//
// The file is expected to contain a two dimensional array of embeddings.
func (r *NpyReader) ReadFloat32() ([][]float32, error) {
	// Open the .npy file
	f, err := os.Open(r.filePath)
	if err != nil {
		sklog.Errorf("could not open %s: %v", r.filePath, err)
		return nil, skerr.Wrap(err)
	}
	defer f.Close()

	// Create a Reader, which reads the header
	reader, err := npyio.NewReader(f)
	if err != nil {
		log.Fatalf("could not create npy reader: %v", err)
	}

	// Figure out the number of rows and columns in the file first.
	// The Shape field contains the array dimensions (e.g., [100, 768])
	shape := reader.Header.Descr.Shape
	if len(shape) != 2 {
		log.Fatalf("expected 2D array, but got %dD array with shape %v", len(shape), shape)
	}

	rows := shape[0]
	cols := shape[1]

	// 2. Now read the full data flattened.
	var data []float32
	err = reader.Read(&data)
	if err != nil {
		sklog.Errorf("could not read %s: %v", r.filePath, err)
		return nil, skerr.Wrap(err)
	}

	// --- Step 3: Reshape the 1D Slice into 2D ---

	// Pre-allocate the result 2D slice
	result2D := make([][]float32, rows)

	// Iterate through rows and slice the flat data
	for i := 0; i < rows; i++ {
		start := i * cols
		end := start + cols
		// Slice the flat data for the current row.
		// Go slices share the underlying array, so this is efficient.
		result2D[i] = data[start:end]
	}

	return result2D, nil
}
