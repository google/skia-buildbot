package bulk

import "go.skia.org/infra/fiddle/go/types"

type Request map[string]types.FiddleContext
type Response map[string]Result

type Result struct {
	RunResults    types.RunResults    `json:"run_results"`
	FiddleContext types.FiddleContext `json:"fiddle_context"`
}

func Run(request Request) (Response, error) {
	resp := Response{}
	// Loop through each request, populate the response.
	for id, req := range request {

	}
	return resp, nil
}

/*

var SkCanvas_MakeRasterDirect_code =
"void draw(SkCanvas* ) {\n" +
"    SkImageInfo info = SkImageInfo::MakeN32Premul(3, 3);  // device aligned, 32 bpp, premultipled\n" +
"    const size_t minRowBytes = info.minRowBytes();  // bytes used by one bitmap row\n" +
"    const size_t size = info.getSafeSize(minRowBytes);  // bytes used by all rows\n" +
"    SkAutoTMalloc<SkPMColor> storage(size);  // allocate storage for pixels\n" +
"    SkPMColor* pixels = storage.get();  // get pointer to allocated storage\n" +
"    // create a SkCanvas backed by a raster device, and delete it when the\n" +
"    // function goes out of scope.\n" +
"    std::unique_ptr<SkCanvas> canvas = SkCanvas::MakeRasterDirect(info, pixels, minRowBytes);\n" +
"    canvas->clear(SK_ColorWHITE);  // white is unpremultiplied, in ARGB order\n" +
"    canvas->flush();  // ensure that pixels are cleared\n" +
"    SkPMColor pmWhite = pixels[0];  // the premultiplied format may vary\n" +
"    SkPaint paint;  // by default, draws black\n" +
"    canvas->drawPoint(1, 1, paint);  // draw in the center\n" +
"    canvas->flush();  // ensure that point was drawn\n" +
"    for (int y = 0; y < info.height(); ++y) {\n" +
"        for (int x = 0; x < info.width(); ++x) {\n" +
"            SkDebugf(\"%c\", *pixels++ == pmWhite ? '-' : 'x');\n" +
"        }\n" +
"        SkDebugf(\"\\n\");\n" +
"    }\n" +
"}\n";

var SkCanvas_MakeRasterDirect_json = {
    "code": SkCanvas_MakeRasterDirect_code,
    "options": {
        "width": 256,
        "height": 256,
        "source": 0,
        "textOnly": true
    },
    "name": "SkCanvas_MakeRasterDirect",
    "overwrite": true
*/
