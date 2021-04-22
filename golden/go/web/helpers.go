package web

import (
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

const (
	contentTypeHeader = "Content-Type"
	jsonContentType   = "application/json"

	accessControlHeader = "Access-Control-Allow-Origin"
	allowAllOrigins     = "*"

	contentTypeOptionsHeader = "X-Content-Type-Options"
	noSniffContent           = "nosniff"
)

// setJSONHeaders sets secure headers for JSON responses.
func setJSONHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set(accessControlHeader, allowAllOrigins)
	h.Set(contentTypeHeader, jsonContentType)
	h.Set(contentTypeOptionsHeader, noSniffContent)
}

// sendJSONResponse serializes resp to JSON. If an error occurs
// a text based error code is send to the client.
func sendJSONResponse(w http.ResponseWriter, resp interface{}) {
	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON response.", http.StatusInternalServerError)
	}
}

// parseJSON extracts the body from the request and parses it into the
// provided interface.
func parseJSON(r *http.Request, v interface{}) error {
	defer util.Close(r.Body)
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}

// encodeImg encodes the given image as a PNG and writes the result to the
// given writer.
func encodeImg(w io.Writer, img *image.NRGBA) error {
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(w, img); err != nil {
		return err
	}
	return nil
}

// decodeImg decodes an image from the given reader and returns it as a NRGBA image.
func decodeImg(reader io.Reader) (*image.NRGBA, error) {
	im, err := png.Decode(reader)
	if err != nil {
		return nil, err
	}
	return diff.GetNRGBA(im), nil
}
