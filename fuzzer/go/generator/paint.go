package generator

import (
	"fmt"
	"math/rand"
	"strconv"
)

type Paint struct {
	Name string
	G    *Writer
}

func (g *Writer) AddPaint() Paint {
	paint := Paint{g.NewVariable("SkPaint", "paint"), g}
	return paint
}

func (p *Paint) set(property, value string) {
	p.G.AddStatement("%s.set%s(%s)", p.Name, property, value)
}

func (p *Paint) SetColor(color string) {
	p.set("Color", color)
}

func (p *Paint) SetAntiAlias(enabled bool) {
	p.set("AntiAlias", fmt.Sprintf("%t", enabled))
}

func (p *Paint) SetStyle(style string) {
	p.set("Style", style)
}

func (p *Paint) SetStrokeWidth(width float32) {
	p.set("StrokeWidth", fmt.Sprintf("%f", width))
}

func (p *Paint) SetStrokeMiter(miter float32) {
	p.set("StrokeMiter", fmt.Sprintf("%f", miter))
}

func (p *Paint) SetStrokeCap(strokeCap string) {
	p.set("StrokeCap", strokeCap)
}

func (p *Paint) SetStrokeJoin(strokeJoin string) {
	p.set("StrokeJoin", strokeJoin)
}

func (p *Paint) setRand(property string, values []string) {
	idx := rand.Intn(len(values))
	p.set(property, values[idx])
}

func (p *Paint) RandColor() {
	color := rand.Uint32()
	p.SetColor(fmt.Sprintf("0x%X", color))
}

func (p *Paint) RandAntiAlias() {
	p.setRand("AntiAlias", []string{"true", "false"})
}

func (p *Paint) RandStyle() {
	p.setRand("Style", []string{"SkPaint::kFill_Style", "SkPaint::kStroke_Style", "SkPaint::kStrokeAndFill_Style"})
}

func (p *Paint) RandStrokeWidth() {
	maxWidthStr := p.G.Config.ExtraParams["MaxStrokeWidth"]
	minWidthStr := p.G.Config.ExtraParams["MinStrokeWidth"]
	maxWidthParsed, err := strconv.ParseFloat(maxWidthStr, 32)
	maxWidth := p.G.FloatMax
	if err == nil {
		maxWidth = float32(maxWidthParsed)
	}
	minWidthParsed, err := strconv.ParseFloat(minWidthStr, 32)
	minWidth := p.G.FloatMin
	if err == nil {
		minWidth = float32(minWidthParsed)
	}
	p.set("StrokeWidth", RandFloatStr(minWidth, maxWidth, true))
}

func (p *Paint) RandStrokeMiter() {
	maxMiterStr := p.G.Config.ExtraParams["MaxStrokeMiter"]
	minMiterStr := p.G.Config.ExtraParams["MinStrokeMiter"]
	maxMiterParsed, err := strconv.ParseFloat(maxMiterStr, 32)
	maxMiter := p.G.FloatMax
	if err == nil {
		maxMiter = float32(maxMiterParsed)
	}
	minMiterParsed, err := strconv.ParseFloat(minMiterStr, 32)
	minMiter := p.G.FloatMin
	if err == nil {
		minMiter = float32(minMiterParsed)
	}
	p.set("StrokeMiter", RandFloatStr(minMiter, maxMiter, true))
}

func (p *Paint) RandStrokeCap() {
	p.setRand("StrokeCap", []string{"SkPaint::kButt_Cap", "SkPaint::kSquare_Cap"})
}

func (p *Paint) RandStrokeJoin() {
	p.setRand("StrokeJoin", []string{"SkPaint::kMiter_Join", "SkPaint::kBevel_Join"})
}

func (p *Paint) Rand() {
	p.RandAntiAlias()
	p.RandStyle()
	p.RandColor()
	p.RandStrokeWidth()
	p.RandStrokeMiter()
	p.RandStrokeCap()
	p.RandStrokeJoin()
}
