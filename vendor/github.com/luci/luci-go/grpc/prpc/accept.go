// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"fmt"
	"mime"
	"strconv"
	"strings"
	"unicode"
)

// This file implements "Accept" HTTP header parser.
// Spec: http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html

// accept is a parsed "Accept" HTTP header.
type accept []acceptType

type acceptType struct {
	MediaType       string
	MediaTypeParams map[string]string
	QualityFactor   float32
	AcceptParams    map[string]string
}

// parseAccept parses an "Accept" HTTP header.
//
// See spec http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html
// Roughly:
// - accept is a list of types separated by ","
// - a type is like media type, except q parameter separates
//   media type parameters and accept parameters.
// - q is quality factor.
//
// This implementation is slow. Does not support accept params.
func parseAccept(v string) (accept, error) {
	if v == "" {
		return nil, nil
	}

	var result accept
	for _, t := range strings.Split(v, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			return nil, fmt.Errorf("no media type")
		}
		mediaType, qValue, _ := qParamSplit(t)
		at := acceptType{QualityFactor: 1.0}
		var err error
		at.MediaType, at.MediaTypeParams, err = mime.ParseMediaType(mediaType)
		if err != nil {
			return nil, fmt.Errorf("%s", strings.TrimPrefix(err.Error(), "mime: "))
		}
		if qValue != "" {
			qualityFactor, err := strconv.ParseFloat(qValue, 32)
			if err != nil {
				return nil, fmt.Errorf("q parameter: expected a floating-point number")
			}
			at.QualityFactor = float32(qualityFactor)
		}
		result = append(result, at)
	}
	return result, nil
}

// qParamSplit splits media type and accept params by "q" parameter.
func qParamSplit(v string) (mediaType string, qValue string, acceptParams string) {
	rest := v
	for {
		semicolon := strings.IndexRune(rest, ';')
		if semicolon < 0 {
			mediaType = v
			return
		}
		semicolonAbs := len(v) - len(rest) + semicolon // mark
		rest = rest[semicolon:]

		rest = rest[1:] // consume ;
		rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
		if rest == "" || (rest[0] != 'q' && rest[0] != 'Q') {
			continue
		}

		rest = rest[1:] // consume q
		rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
		if rest == "" || rest[0] != '=' {
			continue
		}

		rest = rest[1:] // consume =
		rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
		if rest == "" {
			continue
		}

		qValueStartAbs := len(v) - len(rest) // mark
		semicolon2 := strings.IndexRune(rest, ';')
		if semicolon2 >= 0 {
			semicolon2Abs := len(v) - len(rest) + semicolon2
			mediaType = v[:semicolonAbs]
			qValue = v[qValueStartAbs:semicolon2Abs]
			acceptParams = v[semicolon2Abs+1:]
			acceptParams = strings.TrimLeftFunc(acceptParams, unicode.IsSpace)
		} else {
			mediaType = v[:semicolonAbs]
			qValue = v[qValueStartAbs:]
		}
		qValue = strings.TrimRightFunc(qValue, unicode.IsSpace)
		return
	}
}

// acceptFormat is a format specified in "Accept" header.
type acceptFormat struct {
	Format        Format
	QualityFactor float32 // preference, range: [0.0, 0.1]
}

// acceptFormatSlice is sortable by quality factor (desc) and format.
type acceptFormatSlice []acceptFormat

func (s acceptFormatSlice) Len() int {
	return len(s)
}

func (s acceptFormatSlice) Less(i, j int) bool {
	a, b := s[i], s[j]
	const epsilon = 0.000000001
	// quality factor descending
	if a.QualityFactor+epsilon > b.QualityFactor {
		return true
	}
	if a.QualityFactor+epsilon < b.QualityFactor {
		return false
	}
	// format ascending
	return a.Format < b.Format
}

func (s acceptFormatSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
