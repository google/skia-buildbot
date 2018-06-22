// Does a simple sanity check on a locally running fiddler.
package main

import (
	"bytes"
	"encoding/json"

	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

func main() {
	common.Init()
	body := &types.FiddleContext{
		Code: `void draw(SkCanvas* canvas) {
			SkPaint p;
			p.setStyle(SkPaint::kStroke_Style);
			canvas->drawLine(2, 2, 32, 32, p);
		}`,
		Options: types.Options{
			Width:  32,
			Height: 32,
		},
	}
	c := httputils.NewTimeoutClient()
	b, err := json.Marshal(body)
	if err != nil {
		sklog.Error(err)
	}
	r := bytes.NewReader(b)
	resp, err := c.Post("http://localhost:8000", "application/json", r)
	if err != nil || resp.StatusCode != 200 {
		sklog.Fatal("Failed to pass the smoke test.")
	}
	var runResults types.RunResults
	if err := json.NewDecoder(resp.Body).Decode(&runResults); err != nil {
		sklog.Fatal(err)
	}
	if len(runResults.CompileErrors) > 0 || runResults.RunTimeError != "" {
		sklog.Fatalf("Errors building or running: %v", runResults)
	}
}
