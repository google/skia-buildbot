package generator

import (
	"fmt"
)

func (g *Writer) AddPoint() string {
	return g.NewVariable("SkPoint", "point")
}

func (g *Writer) AddPointArray(size int) string {
	return g.NewArray("SkPoint", "point", size)
}

func (g *Writer) AddTDPointArray() string {
	return g.NewVariable("SkTDArray<SkPoint>", "points")
}

func (g *Writer) AddRandPointTDArray() string {
	points := g.AddTDPointArray()
	for i := 0; i < RandInt(1, 10); i++ {
		point := g.AddRandPoint()
		g.AddStatement("*%s.append() = %s", points, point)
	}
	return points
}

func (g *Writer) AddRandPoint() string {
	point := g.AddPoint()
	g.RandPoint(point)
	return point
}

func (g *Writer) AddRandPointArray(size int) string {
	array := g.AddPointArray(size)
	for i := 0; i < size; i++ {
		g.RandPoint(fmt.Sprintf("%s[%d]", array, i))
	}
	return array
}

func (g *Writer) RandPoint(point string) {
	g.AddStatement("%s.fX = %s", point, g.RandScalarStr())
	g.AddStatement("%s.fY = %s", point, g.RandScalarStr())
}
