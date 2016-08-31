// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"fmt"
	"mime"
)

const (
	// ContentTypePRPC is the formal MIME content type for pRPC messages.
	ContentTypePRPC = "application/prpc"
	// ContentTypeJSON is the JSON content type.
	ContentTypeJSON = "application/json"

	mtPRPCEncoding       = "encoding"
	mtPRPCEncodingBinary = "binary"
	mtPRPCEncodingJSONPB = "json"
	mtPRPCEncodingText   = "text"
	mtPRPCBinary         = ContentTypePRPC + "; " + mtPRPCEncoding + "=" + mtPRPCEncodingBinary
	mtPRPCJSONPB         = ContentTypePRPC + "; " + mtPRPCEncoding + "=" + mtPRPCEncodingJSONPB
	mtPRPCText           = ContentTypePRPC + "; " + mtPRPCEncoding + "=" + mtPRPCEncodingText

	// JSONPBPrefix is prepended to a message in JSONPB format to avoid CSRF.
	JSONPBPrefix = ")]}'\n"
)

var bytesJSONPBPrefix = []byte(JSONPBPrefix)

// Format is the pRPC protobuf wire format specification.
type Format int

// (Ordered by preference).
const (
	// FormatBinary indicates that a message is encoded as a raw binary protobuf.
	FormatBinary Format = iota
	// FormatJSONPB indicates that a message is encoded as a JSON-serialized
	// protobuf.
	FormatJSONPB
	// FormatText indicates that a message is encoded as a text protobuf.
	FormatText
)

// FormatFromContentType converts Content-Type header value from a request to a
// format.
// Can return only FormatBinary, FormatJSONPB or FormatText.
// In case of an error, format is undefined.
func FormatFromContentType(v string) (Format, error) {
	if v == "" {
		return FormatBinary, nil
	}
	mediaType, mediaTypeParams, err := mime.ParseMediaType(v)
	if err != nil {
		return 0, err
	}
	return FormatFromMediaType(mediaType, mediaTypeParams)
}

// FormatFromMediaType converts a media type ContentType and its parameters
// into a pRPC Format.
// Can return only FormatBinary, FormatJSONPB or FormatText.
// In case of an error, format is undefined.
func FormatFromMediaType(mt string, params map[string]string) (Format, error) {
	switch mt {
	case ContentTypePRPC:
		for k := range params {
			if k != mtPRPCEncoding {
				return 0, fmt.Errorf("unexpected parameter %q", k)
			}
		}
		return FormatFromEncoding(params[mtPRPCEncoding])

	case ContentTypeJSON:
		return FormatJSONPB, nil

	case "", "*/*", "application/*", "application/grpc":
		return FormatBinary, nil

	default:
		return 0, fmt.Errorf("unknown content type: %q", mt)
	}
}

// FormatFromEncoding converts a media type encoding parameter into a pRPC
// Format.
// Can return only FormatBinary, FormatJSONPB or FormatText.
// In case of an error, format is undefined.
func FormatFromEncoding(v string) (Format, error) {
	switch v {
	case "", mtPRPCEncodingBinary:
		return FormatBinary, nil
	case mtPRPCEncodingJSONPB:
		return FormatJSONPB, nil
	case mtPRPCEncodingText:
		return FormatText, nil
	default:
		return 0, fmt.Errorf(`invalid encoding parameter: %q. Valid values: `+
			`"`+mtPRPCEncodingBinary+`", `+
			`"`+mtPRPCEncodingJSONPB+`", `+
			`"`+mtPRPCEncodingText+`"`, v)
	}
}

// MediaType returns the media type value for a Format.
func (f Format) MediaType() string {
	switch f {
	case FormatJSONPB:
		return mtPRPCEncodingJSONPB
	case FormatText:
		return mtPRPCEncodingText
	case FormatBinary:
		fallthrough
	default:
		return mtPRPCEncodingBinary
	}
}

// ContentType returns a full MIME type for a format.
func (f Format) ContentType() string {
	switch f {
	case FormatJSONPB:
		return mtPRPCJSONPB
	case FormatText:
		return mtPRPCText
	case FormatBinary:
		fallthrough
	default:
		return mtPRPCBinary
	}
}
