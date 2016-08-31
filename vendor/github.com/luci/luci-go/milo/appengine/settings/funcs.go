// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/luci/luci-go/milo/api/resp"
)

// A collection of useful templating functions

// funcMap is what gets fed into the template bundle.
var funcMap = template.FuncMap{
	"humanDuration":  humanDuration,
	"parseRFC3339":   parseRFC3339,
	"linkify":        linkify,
	"obfuscateEmail": obfuscateEmail,
	"localTime":      localTime,
	"shortHash":      shortHash,
	"startswith":     strings.HasPrefix,
	"sub":            sub,
	"consoleHeader":  consoleHeader,
}

// localTime returns a <span> element with t in human format
// that will be converted to local timezone in the browser.
// Recommended usage: {{ .Date | localTime "N/A" }}
func localTime(ifZero string, t time.Time) template.HTML {
	if t.IsZero() {
		return template.HTML(template.HTMLEscapeString(ifZero))
	}
	milliseconds := t.UnixNano() / 1e6
	return template.HTML(fmt.Sprintf(
		`<span class="local-time" data-timestamp="%d">%s</span>`,
		milliseconds,
		t.Format(time.RFC850)))
}

func consoleHeader(brs []resp.BuilderRef) template.HTML {
	// First, split things into nice rows and find the max depth.
	cat := make([][]string, len(brs))
	depth := 0
	for i, b := range brs {
		cat[i] = b.Category
		if len(cat[i]) > depth {
			depth = len(cat[i])
		}
	}

	result := ""
	for row := 0; row < depth; row++ {
		result += "<tr><th></th>"
		// "" is the first node, " " is an empty node.
		current := ""
		colspan := 0
		for _, br := range cat {
			colspan++
			var s string
			if row >= len(br) {
				s = " "
			} else {
				s = br[row]
			}
			if s != current || current == " " {
				if current != "" || current == " " {
					result += fmt.Sprintf(`<th colspan="%d">%s</th>`, colspan, current)
					colspan = 0
				}
				current = s
			}
		}
		if colspan != 0 {
			result += fmt.Sprintf(`<th colspan="%d">%s</th>`, colspan, current)
		}
		result += "</tr>"
	}

	// Last row: The actual builder shortnames.
	result += "<tr><th></th>"
	for _, br := range brs {
		result += fmt.Sprintf("<th>%s</th>", br.ShortName)
	}
	result += "</tr>"
	return template.HTML(result)
}

// humanDuration translates d into a human readable string of x units y units,
// where x and y could be in days, hours, minutes, or seconds, whichever is the
// largest.
func humanDuration(d time.Duration) string {
	t := int64(d.Seconds())
	day := t / 86400
	hr := (t % 86400) / 3600

	if day > 0 {
		if hr != 0 {
			return fmt.Sprintf("%d days %d hrs", day, hr)
		}
		return fmt.Sprintf("%d days", day)
	}

	min := (t % 3600) / 60
	if hr > 0 {
		if min != 0 {
			return fmt.Sprintf("%d hrs %d mins", hr, min)
		}
		return fmt.Sprintf("%d hrs", hr)
	}

	sec := t % 60
	if min > 0 {
		if sec != 0 {
			return fmt.Sprintf("%d mins %d secs", min, sec)
		}
		return fmt.Sprintf("%d mins", min)
	}

	if sec != 0 {
		return fmt.Sprintf("%d secs", sec)
	}

	if d > time.Millisecond {
		return fmt.Sprintf("%d ms", d/time.Millisecond)
	}

	return "0"
}

// obfuscateEmail converts an email@address.com into email<junk>@address.com
// if it is an email.
func obfuscateEmail(email string) template.HTML {
	email = template.HTMLEscapeString(email)
	return template.HTML(strings.Replace(
		email, "@", "<span style=\"display:none\">ohnoyoudont</span>@", -1))
}

// parseRFC3339 parses time represented as a RFC3339 or RFC3339Nano string.
// If cannot parse, returns zero time.
func parseRFC3339(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	t, err = time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t
	}
	return time.Time{}
}

var linkifyTemplate = template.Must(
	template.New("linkify").Parse(`<a href="{{.URL}}">
		{{if .Img}}<img src="{{.Img}}"{{if .Alt}} alt="{{.Alt}}"{{end}}>
		{{else}}{{.Label}}{{end}}</a>`))

// linkify turns a resp.Link struct into a canonical link.
func linkify(link *resp.Link) template.HTML {
	if link == nil {
		return ""
	}
	buf := bytes.Buffer{}
	if err := linkifyTemplate.Execute(&buf, link); err != nil {
		panic(err)
	}
	return template.HTML(buf.Bytes())
}

// sub subtracts one number from another, because apparently go templates aren't
// smart enough to do that.
func sub(a, b int) int {
	return a - b
}

// shortHash abbriviates a git hash into 6 characters.
func shortHash(s string) string {
	if len(s) > 6 {
		return s[0:6]
	}
	return s
}
