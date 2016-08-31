// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package urlparams

import "testing"

func TestParse1(t *testing.T) {
	color, name := "green", "pea"
	vars := Parse("/fruit/:color/:name", "/fruit/"+color+"/"+name)
	if len(vars) != 2 {
		t.Fatalf("Unexpected value. Want 2, have %d", len(vars))
	}
	if vars["color"] != color {
		t.Errorf("Unexpected value. Want %q, have %q", color, vars["color"])
	}
	if vars["name"] != name {
		t.Errorf("Unexpected value. Want %q, have %q", name, vars["name"])
	}
}

func TestParse2(t *testing.T) {
	color, name := "green", "peas"
	vars := Parse("/fruit/:color/all/:name", "/fruit/"+color+"/all/"+name)
	if len(vars) != 2 {
		t.Fatalf("Unexpected value. Want 2, have %d", len(vars))
	}
	if vars["color"] != color {
		t.Errorf("Unexpected value. Want %q, have %q", color, vars["color"])
	}
	if vars["name"] != name {
		t.Errorf("Unexpected value. Want %q, have %q", name, vars["name"])
	}
}

func BenchmarkParse(b *testing.B) {
	for n := 0; n < b.N; n++ {
		Parse("/fruit/:color/:name", "/fruit/green/pea")
	}
}
