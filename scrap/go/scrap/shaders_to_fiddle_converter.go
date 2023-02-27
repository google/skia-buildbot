package scrap

import (
	"bytes"
	"io"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// extractBodyUniforms will parse the SkSL code in a scrap and extract definitions
// for each uniform defined within.
func extractBodyUniforms(body ScrapBody) ([]uniformValue, error) {
	var uniforms []uniformValue
	for _, line := range strings.Split(body.Body, "\n") {
		if m := uniformDefinitionRegex.FindStringSubmatch(line); m != nil {
			uniforms = append(uniforms, uniformValue{
				Name: m[2],
				Type: m[1],
			})
		}
	}
	return uniforms, nil
}

// writeShaderProgramAsCPPStringConstant writes the shader code into a C++
// string constant.
func writeShaderProgramAsCPPStringConstant(w io.StringWriter, node scrapNode) {
	mustWriteStringf(w, "    constexpr char prog%s[] = R\"(\n", node.Name)

	mustWriteStringf(w, indentMultilineString(skslDefaultInputs, 8))

	mustWriteStringf(w, "\n")
	writeShaderInputDefinitions(w, node, 8)
	mustWriteStringf(w, "\n")
	mustWriteStringf(w, indentMultilineString(node.Scrap.Body, 8))

	mustWriteStringf(w, "\n    )\";")
}

// writeCustomUniformSetup will write the Skia C++ code to the supplied
// StringWriter to initialize any custom uniform values defined in the
// scrap.
func writeCustomUniformSetup(w io.StringWriter, node scrapNode) error {
	if node.Scrap.Type != SKSL || node.Scrap.SKSLMetaData == nil || len(node.Scrap.SKSLMetaData.Uniforms) == 0 {
		return nil
	}
	uniforms, err := extractBodyUniforms(node.Scrap)
	if err != nil {
		return skerr.Wrap(err)
	}
	requiredFloatCount := 0
	for _, u := range uniforms {
		num, err := u.numFloats()
		if err != nil {
			return skerr.Wrap(err)
		}
		requiredFloatCount += num
	}
	if requiredFloatCount != len(node.Scrap.SKSLMetaData.Uniforms) {
		return skerr.Fmt("user inputs size mismatch: %d != %d.",
			requiredFloatCount, len(node.Scrap.SKSLMetaData.Uniforms))
	}
	i := 0
	mustWriteStringf(w, "    // Inputs supplied by user:\n")
	for _, u := range uniforms {
		s, err := u.getCppDefinitionString(node.Scrap.SKSLMetaData.Uniforms[i:], node.Name)
		if err != nil {
			return skerr.Fmt("cannot get C++ definition for %s (%s).", u.Name, u.Type)
		}
		mustWriteStringf(w, "    %s\n", s)
		num, _ := u.numFloats()
		i += num
	}
	return nil
}

func writeShaderUniformSetup(w io.StringWriter, node scrapNode) error {
	if node.Scrap.Type != SKSL || node.Scrap.SKSLMetaData == nil {
		return nil
	}

	if len(node.Scrap.SKSLMetaData.Children) != len(node.Children) {
		// This is a program error. They should be the same size and order.
		return skerr.Fmt("Child count mismatch")
	}
	for i := 0; i < len(node.Children); i++ {
		mustWriteStringf(w, "    builder%s.child(\"%s\") = shader%s;\n",
			node.Name,
			node.Scrap.SKSLMetaData.Children[i].UniformName,
			node.Children[i].Name,
		)
	}

	return nil
}

func writeShaderCreationCPP(w io.StringWriter, node scrapNode) error {
	mustWriteStringf(w, "    auto [effect%s, err%s] = SkRuntimeEffect::MakeForShader(SkString(prog%s));\n", node.Name, node.Name, node.Name)
	mustWriteStringf(w, "    if (!effect%s) {\n", node.Name)
	mustWriteStringf(w, "        SkDebugf(\"Cannot create effect%s\");\n", node.Name)
	mustWriteStringf(w, "        return;\n")
	mustWriteStringf(w, "    }\n")
	mustWriteStringf(w, "    SkRuntimeShaderBuilder builder%s(effect%s);\n", node.Name, node.Name)
	mustWriteStringf(w, "    builder%s.uniform(\"iResolution\") = viewportResolution;\n", node.Name)
	mustWriteStringf(w, "    builder%s.uniform(\"iTime\") = playbackTime;\n", node.Name)
	mustWriteStringf(w, "    builder%s.uniform(\"iMouse\") = mousePos;\n", node.Name)
	mustWriteStringf(w, "    builder%s.uniform(\"iImageResolution\") =\n", node.Name)
	mustWriteStringf(w, "        SkV3{static_cast<float>(image->width()),\n")
	mustWriteStringf(w, "             static_cast<float>(image->height()), 1.0f};\n")
	mustWriteStringf(w, "    builder%s.child(\"iImage1\") = image->makeShader(shaderOptions);\n", node.Name)
	if err := writeCustomUniformSetup(w, node); err != nil {
		return skerr.Wrap(err)
	}
	if err := writeShaderUniformSetup(w, node); err != nil {
		return skerr.Wrap(err)
	}
	mustWriteStringf(w, "    sk_sp<SkShader> shader%s = builder%s.makeShader();\n", node.Name, node.Name)
	mustWriteStringf(w, "    if (!shader%s) {\n", node.Name)
	mustWriteStringf(w, "        SkDebugf(\"Cannot create shader%s\");\n", node.Name)
	mustWriteStringf(w, "        return;\n")
	mustWriteStringf(w, "    }\n")
	return nil
}

// writeCreateShadersCPP writes C++ code need to create the
// runtime effect for the scrap |node| and all child nodes to |w|.
func writeCreateShadersCPP(w io.StringWriter, node scrapNode) error {
	for _, child := range node.Children {
		mustWriteStringf(w, "\n")
		if err := writeCreateShadersCPP(w, child); err != nil {
			return skerr.Wrap(err)
		}
	}
	mustWriteStringf(w, "\n")
	if node.Name != "" {
		mustWriteStringf(w, "    // Shader %q:\n", node.Name)
	}
	writeShaderProgramAsCPPStringConstant(w, node)
	mustWriteStringf(w, "\n")
	if err := writeShaderCreationCPP(w, node); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// createShadersCPP is the template.Template callback to write C++ code to create
// all shaders for the given |root| node and all child nodes.
func createShadersCPP(root scrapNode) (string, error) {
	var b bytes.Buffer
	if err := writeCreateShadersCPP(&b, root); err != nil {
		return "", skerr.Wrap(err)
	}
	return b.String(), nil
}

const skslCpp = `void draw(SkCanvas *canvas) {
    constexpr SkV4 mousePos = SkV4{0.0f, 0.0f, 0.0f, 0.0f};
    constexpr SkV3 viewportResolution = SkV3{256, 256, 1.0f};
    const SkSamplingOptions shaderOptions(SkFilterMode::kLinear);
    const float playbackTime = duration != 0.0 ? frame * duration : 0.0;
{{ createShadersCPP . }}
    // Fill the surface with |shader{{ .Name }}|:
    SkPaint p;
    p.setShader(shader{{ .Name }});
    canvas->drawPaint(p);
}`
