/* PDF Rasterizer Package

This package provides the Pdfium and Poppler rasterizers.  Pdfium
requires the `pdfium_test` executable be located in the PATH.  Poppler
requires `pdftoppm` (provided by poppler-utils) and `pnmtopng`
(provided by netpbm). */

package pdf

type Rasterizer interface {
	// Rasterize will take the path to a PDF file and rasterize the
	// file.  If the file has multiple pages, discard all but the first
	// page.  The output file will be in PNG format.
	Rasterize(pdfInputPath, pngOutputPath string) error
	// Return the name of this rasterizer.
	String() string
	// Return false if the rasterizer is found.
	Enabled() bool
}
