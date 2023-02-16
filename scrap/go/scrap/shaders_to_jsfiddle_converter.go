package scrap

import (
	"bytes"
	"fmt"
	"io"

	"go.skia.org/infra/go/skerr"
)

// writeInfo contains data for of a single template execution of a scrap node tree.
type writeInfo struct {
	imageIDs map[string]int // Map image URLs to image number.
}

// newWriteInfo creates a writeInfo populated with information
// needed to fill a template during execution.
func newWriteInfo(root scrapNode) (writeInfo, error) {
	var ret writeInfo
	urls, err := root.getImageURLs()
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	ret.imageIDs = make(map[string]int)
	for i, url := range urls {
		ret.imageIDs[url] = i + 1
	}
	return ret, nil
}

// getNodeImageID will return the image ID that will be used by a shader scrap.
func getNodeImageID(body ScrapBody, info writeInfo) (int, error) {
	url, err := getSkSLImageURL(body)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	imgNum, ok := info.imageIDs[url]
	if !ok {
		return 0, skerr.Fmt("Cannot find index of scrap image url %q", url)
	}
	return imgNum, nil
}

// loadImagesJS creates the JavaScript promises to load
// all images used by the |root| scrap, and all child scraps.
func loadImagesJS(root scrapNode) (string, error) {
	urls, err := root.getImageURLs()
	if err != nil {
		return "", skerr.Wrap(err)
	}
	info, err := newWriteInfo(root)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Write image fetch promises for each unique image in the scrap node tree:
	//
	// const loadImageX = fetch(url)
	//     .then((response) => response.arrayBuffer());
	var b bytes.Buffer
	for _, url := range urls {
		imgID, ok := info.imageIDs[url]
		if !ok {
			return "", skerr.Fmt("Cannot find id for img url %q", url)
		}
		mustWriteStringf(&b, "const loadImage%d = fetch(\"%s\")\n", imgID, url)
		mustWriteStringf(&b, "  .then((response) => response.arrayBuffer());\n")
	}
	mustWriteStringf(&b, "\n")

	// Write promise waiter:
	//
	// Promise.all([loadImage1, ..., loadImageN]).then((values) => {
	mustWriteStringf(&b, "Promise.all([")
	for i, url := range urls {
		imgID, ok := info.imageIDs[url]
		if !ok {
			return "", skerr.Fmt("Cannot find id for img url %q", url)
		}
		mustWriteStringf(&b, "loadImage%d", imgID)
		if i < len(urls)-1 {
			mustWriteStringf(&b, ", ")
		}
	}
	mustWriteStringf(&b, "]).then((values) => {\n")

	// Values:
	//
	// const [imageData1, ..., imageDataN] = values;
	mustWriteStringf(&b, "  const [")
	for i, url := range urls {
		imgID, ok := info.imageIDs[url]
		if !ok {
			return "", skerr.Fmt("Cannot find id for img url %q", url)
		}
		mustWriteStringf(&b, "imageData%d", imgID)
		if i < len(urls)-1 {
			mustWriteStringf(&b, ", ")
		}
	}
	mustWriteStringf(&b, "] = values;\n")

	// Create shaders:
	//
	// const imgX = CanvasKit.MakeImageFromEncoded(imageDataX);
	// const imgShaderX = imgX.makeShaderCubic(
	//    CanvasKit.TileMode.Clamp, CanvasKit.TileMode.Clamp, 1/3, 1/3);
	for i := 1; i <= len(urls); i++ {
		mustWriteStringf(&b, "  const img%d = CanvasKit.MakeImageFromEncoded(imageData%d);\n", i, i)
		mustWriteStringf(&b, "  const imgShader%d = img%d.makeShaderCubic(\n", i, i)
		mustWriteStringf(&b, "    CanvasKit.TileMode.Clamp, CanvasKit.TileMode.Clamp, 1/3, 1/3);\n")
	}
	return b.String(), nil
}

// writeCreateRuntimeEffects writes JavaScript code need to create the
// runtime effect for the scrap |node| and all child nodes to |w|.
func writeCreateRuntimeEffects(w io.StringWriter, node scrapNode) {
	for _, child := range node.Children {
		writeCreateRuntimeEffects(w, child)
		mustWriteStringf(w, "\n")
	}
	mustWriteStringf(w, "\n")
	if node.Name != "" {
		mustWriteStringf(w, "  // Shader %q\n", node.Name)
	}
	mustWriteStringf(w, "  const prog%s = `\n", node.Name)
	mustWriteStringf(w, indentMultilineString(skslDefaultInputs, 4))
	mustWriteStringf(w, "\n")
	writeShaderInputDefinitions(w, node, 4)
	mustWriteStringf(w, "\n")
	mustWriteStringf(w, indentMultilineString(node.Scrap.Body, 4))
	mustWriteStringf(w, "\n    `;\n")

	mustWriteStringf(w, "  const effect%s = CanvasKit.RuntimeEffect.Make(prog%s);", node.Name, node.Name)
}

// createRuntimeEffectsJS is the template.Template callback to write
// JavaScript code to create all effects for the given |root| node
// and all child nodes.
func createRuntimeEffectsJS(root scrapNode) (string, error) {
	var b bytes.Buffer
	writeCreateRuntimeEffects(&b, root)
	return b.String(), nil
}

// writeUniformArray writes JavaScript code to define an array with
// all uniform values (stock and custom) for the given |node|.
func writeUniformArray(w io.StringWriter, node scrapNode, imgID int) {

	mustWriteStringf(w, "    const uniforms%s = [\n", node.Name)
	mustWriteStringf(w, "      shaderWidth, shaderHeight, 1,                     // iResolution\n")
	mustWriteStringf(w, "      iTime,                                            // iTime\n")
	mustWriteStringf(w, "      mouseDragX, mouseDragY, mouseClickX, mouseClickY, // iMouse\n")
	mustWriteStringf(w, "      img%d.width(), img%d.height(), 1,                   // iImageResolution\n", imgID, imgID)

	if u := getSkSLCustomUniforms(node.Scrap); u != "" {
		mustWriteStringf(w, "%s\n", u)
	}
	mustWriteStringf(w, "    ];\n")
}

// writeChildrenArray writes JavaScript code to define an array with
// all children (stock and custom) for the given |node|.
func writeChildrenArray(w io.StringWriter, node scrapNode, imgID int) {
	mustWriteStringf(w, "    const children%s = [\n", node.Name)

	mustWriteStringf(w, "      imgShader%d,                                       // iImage1\n", imgID)
	for _, child := range node.Children {
		mustWriteStringf(w, "      shader%s,\n", child.Name)
	}

	mustWriteStringf(w, "    ];\n")
}

// writeCodeToCreateShader writes JavaScript code to create all shaders
// for a given |node| and all child nodes.
func writeCodeToCreateShader(w io.StringWriter, node scrapNode, info writeInfo) error {
	imgID, err := getNodeImageID(node.Scrap, info)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, child := range node.Children {
		if err := writeCodeToCreateShader(w, child, info); err != nil {
			return skerr.Wrap(err)
		}
		mustWriteStringf(w, "\n")
	}
	writeUniformArray(w, node, imgID)
	writeChildrenArray(w, node, imgID)

	mustWriteStringf(w,
		"    const shader%s = effect%s.makeShaderWithChildren(uniforms%s, children%s);\n",
		node.Name, node.Name, node.Name, node.Name)
	mustWriteStringf(w, "    if (!shader%s) {\n", node.Name)
	mustWriteStringf(w, "      throw \"Could not make shader%s\";\n", node.Name)
	mustWriteStringf(w, "    }")

	return nil
}

// createFragmentShadersJS is a Template callback to insert the JavaScript
// code to create all shaders for the |root| node and all child nodes.
func createFragmentShadersJS(root scrapNode) (string, error) {
	info, err := newWriteInfo(root)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	var b bytes.Buffer
	if err := writeCodeToCreateShader(&b, root, info); err != nil {
		return "", skerr.Wrap(err)
	}
	return b.String(), nil
}

// Write JavaScript code to set the CanvasKit.Paint shader for
// the given |node|.
func putShaderOnPaintJS(node scrapNode) string {
	return fmt.Sprintf("paint.setShader(shader%s);", node.Name)
}

// Write JavaScript code to delete the shader for the given |node|
// and all child nodes.
func deleteShadersJS(node scrapNode) string {
	var b bytes.Buffer
	for _, child := range node.Children {
		mustWriteStringf(&b, deleteShadersJS(child))
		mustWriteStringf(&b, "\n")
	}
	mustWriteStringf(&b, "    shader%s.delete();", node.Name)
	return b.String()
}

// The template used to convert a SkSL shader scrap (from shaders.skia.org)
// to JavaScript suitable for direct use in jsfiddle.skia.org.
const skslJavaScript = `const shaderWidth = 512;
const shaderHeight = 512;
{{ loadImagesJS . }}
  const surface = CanvasKit.MakeCanvasSurface(canvas.id);
  if (!surface) {
    throw "Could not make surface";
  }
  const skcanvas = surface.getCanvas();
  const paint = new CanvasKit.Paint();
  const startTimeMs = Date.now();
  let mouseClickX = 0;
  let mouseClickY = 0;
  let mouseDragX = 0;
  let mouseDragY = 0;
  let lastMousePressure = 0;
{{ createRuntimeEffectsJS . }}

  function drawFrame(canvas) {
    const iTime = (Date.now() - startTimeMs) / 1000;
{{ createFragmentShadersJS . }}
    {{ putShaderOnPaintJS . }}
    skcanvas.drawPaint(paint);
{{ deleteShadersJS . }}
    surface.requestAnimationFrame(drawFrame);
  }
  surface.requestAnimationFrame(drawFrame);

  canvas.addEventListener("pointermove", (e) => {
    if (e.pressure && !lastMousePressure) {
      mouseClickX = e.offsetX;
      mouseClickY = e.offsetY;
    }
    lastMousePressure = e.pressure;
    if (!e.pressure) {
      return;
    }
    mouseDragX = e.offsetX;
    mouseDragY = e.offsetY;
  });
}); // from the Promise.all
`
