package generator

import (
	"math/rand"
)

type Path struct {
	Name                                   string
	DepthLimit, ContourLimit, SegmentLimit int
	G                                      *Writer
}

func (g *Writer) AddPath() Path {
	path := Path{g.NewVariable("SkPath", "path"), 0, 0, 0, g}
	return path
}

func (p *Path) AddMoveTo() {
	p.G.AddStatement("%s.moveTo(%s, %s)", p.Name, p.G.RandScalarStr(), p.G.RandScalarStr())
}

func (p *Path) AddRMoveTo() {
	p.G.AddStatement("%s.rMoveTo(%s, %s)", p.Name, p.G.RandScalarStr(), p.G.RandScalarStr())
}

func (p *Path) AddLineTo() {
	p.G.AddStatement("%s.lineTo(%s, %s)", p.Name, p.G.RandScalarStr(), p.G.RandScalarStr())
}

func (p *Path) AddRLineTo() {
	p.G.AddStatement("%s.rLineTo(%s, %s)", p.Name, p.G.RandScalarStr(), p.G.RandScalarStr())
}

func (p *Path) AddQuadTo() {
	array := p.G.AddRandPointArray(2)
	p.G.AddStatement("%s.quadTo(%s[0], %s[1])", p.Name, array, array)
}

func (p *Path) AddRQuadTo() {
	array := p.G.AddRandPointArray(2)
	p.G.AddStatement("%s.quadTo(%s[0].fX, %s[0].fY, %s[1].fX, %s[1].fY)", p.Name, array, array, array, array)
}

func (p *Path) AddCubicTo() {
	array := p.G.AddRandPointArray(3)
	p.G.AddStatement("%s.cubicTo(%s[0], %s[1], %s[2])", p.Name, array, array, array)
}

func (p *Path) AddRCubicTo() {
	array := p.G.AddRandPointArray(3)
	p.G.AddStatement("%s.rCubicTo(%s[0].fX, %s[0].fY, %s[1].fX, %s[1].fY, %s[2].fX, %s[2].fY)", p.Name, array, array, array, array, array, array)
}

func (p *Path) AddConicTo() {
	array := p.G.AddRandPointArray(2)
	p.G.AddStatement("%s.conicTo(%s[0], %s[1], %s)", p.Name, array, array, p.G.RandScalarStr())
}

func (p *Path) AddRConicTo() {
	array := p.G.AddRandPointArray(2)
	p.G.AddStatement("%s.rConicTo(%s[0].fX, %s[0].fY, %s[1].fX, %s[1].fY, %s)", p.Name, array, array, array, array, p.G.RandScalarStr())
}

func (p *Path) AddArcTo() {
	array := p.G.AddRandPointArray(2)
	p.G.AddStatement("%s.arcTo(%s[0], %s[1], %s)", p.Name, array, array, p.G.RandScalarStr())
}

func (p *Path) AddClose() {
	p.G.AddStatement("%s.close()", p.Name)
}

func (p *Path) AddArcTo2() {
	oval := p.G.AddRandRect()
	startAngle := p.G.RandAngleStr()
	sweepAngle := p.G.RandAngleStr()
	forceMoveTo := p.G.RandBoolStr()

	p.G.AddStatement("%s.arcTo(%s, %s, %s, %s)", p.Name, oval.Name, startAngle, sweepAngle, forceMoveTo)
}

func (p *Path) AddArc() {
	oval := p.G.AddRandRect()
	startAngle := p.G.RandAngleStr()
	sweepAngle := p.G.RandAngleStr()

	p.G.AddStatement("%s.addArc(%s, %s, %s)", p.Name, oval.Name, startAngle, sweepAngle)
}

func (p *Path) AddPoly() {
	points := p.G.AddRandPointTDArray()
	p.G.AddStatement("%s.addPoly(&%s[0], %s.count(), %s)", p.Name, points, points, p.G.RandBoolStr())
}

func (p *Path) AddRoundRect1() {
	rect := p.G.AddRandRect()
	rx := p.G.RandScalarStr()
	ry := p.G.RandScalarStr()
	dir := p.G.RandDirectionStr()

	p.G.AddStatement("%s.addRoundRect(%s, %s, %s, %s)", p.Name, rect.Name, rx, ry, dir)
}

func (p *Path) AddRoundRect2() {
	rect := p.G.AddRandRect()
	radii := p.G.AddRandScalarArray("radii", 8)
	dir := p.G.RandDirectionStr()

	p.G.AddStatement("%s.addRoundRect(%s, %s, %s)", p.Name, rect.Name, radii, dir)
}

func (p *Path) AddRRect() {
	rrect := p.G.AddRandRRect()
	dir := p.G.RandDirectionStr()

	p.G.AddStatement("%s.addRRect(%s, %s)", p.Name, rrect.Name, dir)
}

func (p *Path) AddPath1() {
	if p.DepthLimit > 0 {
		subpath := p.G.AddPath()
		subpath.Rand(p.DepthLimit-1, p.ContourLimit, p.SegmentLimit)
		mode := p.G.RandAddPathModeStr()
		dx := p.G.RandScalarStr()
		dy := p.G.RandScalarStr()
		p.G.AddStatement("%s.addPath(%s, %s, %s, %s)", p.Name, subpath.Name, dx, dy, mode)
	}
}

func (p *Path) AddPath2() {
	if p.DepthLimit > 0 {
		subpath := p.G.AddPath()
		subpath.Rand(p.DepthLimit-1, p.ContourLimit, p.SegmentLimit)
		mode := p.G.RandAddPathModeStr()
		p.G.AddStatement("%s.addPath(%s, %s)", p.Name, subpath.Name, mode)
	}
}

func (p *Path) AddPath3() {
	if p.DepthLimit > 0 {
		subpath := p.G.AddPath()
		subpath.Rand(p.DepthLimit-1, p.ContourLimit, p.SegmentLimit)
		mode := p.G.RandAddPathModeStr()
		matrix := p.G.AddMatrix()
		matrix.Rand()
		p.G.AddStatement("%s.addPath(%s, %s, %s)", p.Name, subpath.Name, matrix.Name, mode)
	}
}

func (p *Path) ReverseAddPath() {
	if p.DepthLimit > 0 {
		subpath := p.G.AddPath()
		subpath.Rand(p.DepthLimit-1, p.ContourLimit, p.SegmentLimit)
		p.G.AddStatement("%s.reverseAddPath(%s)", p.Name, subpath.Name)
	}
}

func (p *Path) Rand(depthLimit, contourLimit, segmentLimit int) {
	p.DepthLimit = depthLimit
	p.ContourLimit = contourLimit
	p.SegmentLimit = segmentLimit
	p.G.AddStatement("%s.reset()", p.Name)

	choices := []func(){
		p.AddMoveTo,
		p.AddRMoveTo,
		p.AddLineTo,
		p.AddRLineTo,
		p.AddQuadTo,
		p.AddRQuadTo,
		p.AddCubicTo,
		p.AddRCubicTo,
		p.AddArcTo,
		p.AddArcTo2,
		p.AddClose,
		p.AddArc,
		p.AddRoundRect1,
		p.AddRoundRect2,
		p.AddRRect,
		p.AddPoly,
		p.AddPath1,
		p.AddPath2,
		p.AddPath3,
		p.AddConicTo,
		p.AddRConicTo,
		p.ReverseAddPath,
	}

	for cIndex := 0; cIndex < contourLimit; cIndex++ {
		segments := RandInt(1, segmentLimit)
		for sIndex := 0; sIndex < segments; sIndex++ {
			addSegmentFunc := choices[rand.Intn(len(choices))]
			addSegmentFunc()
		}
	}
}
