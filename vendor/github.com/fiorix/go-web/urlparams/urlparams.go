// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Parser for URLs containing variable arguments.
package urlparams

import "strings"

// Parse extracts parameters from url based on the format and returns a map
// of those values.
//
// e.g. Parse("/fruit/:color/:name", "/fruit/green/pea") returns a map
// containing color="green" and "name"="pea".
func Parse(format, url string) map[string]string {
	vars := make(map[string]string)
	fmtParts, urlParts := strings.Split(format, "/"), strings.Split(url, "/")
	for i := 0; i < len(fmtParts) && i < len(urlParts); i++ {
		if len(fmtParts[i]) > 0 && fmtParts[i][0] == ':' {
			vars[fmtParts[i][1:]] = urlParts[i]
		}
	}
	return vars
}
