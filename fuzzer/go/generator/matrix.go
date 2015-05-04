package generator

import (
	"math/rand"
)

type Matrix struct {
	Name string
	G    *Writer
}

func (g *Writer) AddMatrix() Matrix {
	matrix := Matrix{g.NewVariable("SkMatrix", "matrix"), g}
	return matrix
}

func (m *Matrix) SetIdentity() {
	m.G.AddStatement(m.Name + " = SkMatrix::I()")
}

func (m *Matrix) SetTranslateX(value string) {
	m.G.AddStatement("%s.setTranslateX(%s)", m.Name, value)
}

func (m *Matrix) SetTranslateY(value string) {
	m.G.AddStatement("%s.setTranslateY(%s)", m.Name, value)
}

func (m *Matrix) SetTranslate(xValue, yValue string) {
	m.G.AddStatement("%s.setTranslate(%s,%s)", m.Name, xValue, yValue)
}

func (m *Matrix) SetScaleX(value string) {
	m.G.AddStatement("%s.setScaleX(%s)", m.Name, value)
}

func (m *Matrix) SetScaleY(value string) {
	m.G.AddStatement("%s.setScaleY(%s)", m.Name, value)
}

func (m *Matrix) SetScale(xValue, yValue string) {
	m.G.AddStatement("%s.setScale(%s,%s)", m.Name, xValue, yValue)
}

func (m *Matrix) SetScaleTranslate(xScale, yScale, xTranslate, yTranslate string) {
	m.G.AddStatement("%s.setScale(%s,%s,%s,%s)", m.Name, xScale, yScale, xTranslate, yTranslate)
}

func (m *Matrix) SetSkewX(value string) {
	m.G.AddStatement("%s.setSkewX(%s)", m.Name, value)
}

func (m *Matrix) SetSkewY(value string) {
	m.G.AddStatement("%s.setSkewY(%s)", m.Name, value)
}

func (m *Matrix) SetSkew(xValue, yValue string) {
	m.G.AddStatement("%s.setSkew(%s,%s)", m.Name, xValue, yValue)
}

func (m *Matrix) SetSkewTranslate(xSkew, ySkew, xTranslate, yTranslate string) {
	m.G.AddStatement("%s.setSkew(%s,%s,%s,%s)", m.Name, xSkew, ySkew, xTranslate, yTranslate)
}

func (m *Matrix) SetRotate(value string) {
	m.G.AddStatement("%s.setRotate(%s)", m.Name, value)
}

func (m *Matrix) SetRotateTranslate(angle, xTranslate, yTranslate string) {
	m.G.AddStatement("%s.setRotate(%s,%s,%s)", m.Name, angle, xTranslate, yTranslate)
}

func (m *Matrix) SetPerspX(value string) {
	m.G.AddStatement("%s.setPerspX(%s)", m.Name, value)
}

func (m *Matrix) SetPerspY(value string) {
	m.G.AddStatement("%s.setPerspY(%s)", m.Name, value)
}

func (m *Matrix) SetAll(v1, v2, v3, v4, v5, v6, v7, v8, v9 string) {
	m.G.AddStatement("%s.setAll(%s,%s,%s,%s,%s,%s,%s,%s,%s)", m.Name, v1, v2, v3, v4, v5, v6, v7, v8, v9)
}

func (m *Matrix) RandTranslateX() {
	m.SetTranslateX(m.G.RandScalarStr())
}

func (m *Matrix) RandTranslateY() {
	m.SetTranslateY(m.G.RandScalarStr())
}

func (m *Matrix) RandTranslate() {
	m.SetTranslate(m.G.RandScalarStr(), m.G.RandScalarStr())
}

func (m *Matrix) RandScaleX() {
	m.SetScaleX(m.G.RandScalarStr())
}

func (m *Matrix) RandScaleY() {
	m.SetScaleY(m.G.RandScalarStr())
}

func (m *Matrix) RandScale() {
	m.SetScale(m.G.RandScalarStr(), m.G.RandScalarStr())
}

func (m *Matrix) RandScaleTranslate() {
	m.SetScaleTranslate(m.G.RandScalarStr(), m.G.RandScalarStr(), m.G.RandScalarStr(), m.G.RandScalarStr())
}

func (m *Matrix) RandSkewX() {
	m.SetSkewX(m.G.RandScalarStr())
}

func (m *Matrix) RandSkewY() {
	m.SetSkewY(m.G.RandScalarStr())
}

func (m *Matrix) RandSkew() {
	m.SetSkew(m.G.RandScalarStr(), m.G.RandScalarStr())
}

func (m *Matrix) RandSkewTranslate() {
	m.SetSkewTranslate(m.G.RandScalarStr(), m.G.RandScalarStr(), m.G.RandScalarStr(), m.G.RandScalarStr())
}

func (m *Matrix) RandRotate() {
	m.SetRotate(m.G.RandScalarStr())
}

func (m *Matrix) RandRotateTranslate() {
	m.SetRotateTranslate(m.G.RandScalarStr(), m.G.RandScalarStr(), m.G.RandScalarStr())
}

func (m *Matrix) RandPerspX() {
	m.SetPerspX(m.G.RandScalarStr())
}

func (m *Matrix) RandPerspY() {
	m.SetPerspY(m.G.RandScalarStr())
}

func (m *Matrix) RandAll() {
	m.SetAll(m.G.RandScalarStr(), m.G.RandScalarStr(), m.G.RandScalarStr(),
		m.G.RandScalarStr(), m.G.RandScalarStr(), m.G.RandScalarStr(),
		m.G.RandScalarStr(), m.G.RandScalarStr(), m.G.RandScalarStr())
}

func (m *Matrix) Rand() {
	m.G.AddStatement("%s.reset()", m.Name)
	choices := []func(){
		m.SetIdentity,
		m.RandTranslateX,
		m.RandTranslateY,
		m.RandTranslate,
		m.RandScaleX,
		m.RandScaleY,
		m.RandScale,
		m.RandScaleTranslate,
		m.RandSkewX,
		m.RandSkewY,
		m.RandSkew,
		m.RandSkewTranslate,
		m.RandRotate,
		m.RandRotateTranslate,
		m.RandPerspX,
		m.RandPerspY,
		m.RandAll,
	}

	idx := rand.Intn(len(choices))
	choices[idx]()
}
