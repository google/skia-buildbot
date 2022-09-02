package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// testCode is fragments of C++ code we should be able to extract
// comments from, and some others we shouldn't.
const testCode = `

/*
 * Copyright 2015 Google Inc.
 *
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */
DEF_SIMPLE_GM(gamma, canvas, 560, 200) {
    }


// This GM renders correctly in 8888, but fails in PDF
DEF_SIMPLE_GM(fadefilter, canvas, 256, 256) {
    SkScalar matrix[20] = { 1, 0, 0, 0, 128.0f,
                            0, 1, 0, 0, 128.0f,
                            0, 0, 1, 0, 128.0f,



  // should draw only green
DEF_SIMPLE_GM(small_color_stop, canvas, 100, 150) {
    SkColor colors[] = { SK_ColorGREEN, SK_ColorRED, SK_ColorYELLOW };
    SkScalar pos[] = { 0, 0.003f, SK_Scalar1 };  // 0.004f makes this work
    SkPoint c0 = { 200, 25 };
    SkScalar r0 = 20;
    SkPoint c1 = { 200, 25 };


/*
 *  Exercise duplicate color-stops, at the ends, and in the middle
 *
 *  At the time of this writing, only Linear correctly deals with duplicates at the ends,
 *  and then only correctly on CPU backend.
 */
DEF_SIMPLE_GM(gradients_dup_color_stops, canvas, 704, 564) {
    const SkColor preColor  = 0xFFFF0000;   // clamp color before start
    const SkColor postColor = 0xFF0000FF;   // clamp color after end
    const SkColor color0    = 0xFF000000;


DEF_SIMPLE_GM(gradients_dup_color_ignored, canvas, 704, 564) {
    const SkColor preColor  = 0xFFFF0000;


// Stray comments shouldn't match.


// A blank line should also not match.

DEF_SIMPLE_GM(gradients_dup_color_spaced out, canvas, 704, 564) {
    const SkColor preColor  = 0xFFFF0000;



// Multiple lines
// should be concatenated
// into a single comment.
DEF_SIMPLE_GM(test_multiple_single_line_comments, canvas, 704, 564) {


// Names include 0-9.
//
DEF_SIMPLE_GM(blur2rects, canvas, 700, 500) {
        SkPaint paint;

/* Begin and end comment on the same line. */
DEF_SIMPLE_GM(arccirclegap, canvas, 250, 250) {
`

func TestExtract(t *testing.T) {
	gms := Extract(testCode, "filename.cpp")
	assert.Equal(t, 7, len(gms))

	assert.Equal(t, "gamma", gms[0].Name)
	assert.Equal(t, 8, gms[0].Line)
	assert.Equal(t, "filename.cpp", gms[0].Filename)
	assert.Equal(t, "fadefilter", gms[1].Name)
	assert.Equal(t, "small_color_stop", gms[2].Name)
	assert.Equal(t, "gradients_dup_color_stops", gms[3].Name)
	assert.Equal(t, "test_multiple_single_line_comments", gms[4].Name)
	assert.Equal(t, "blur2rects", gms[5].Name)
	assert.Equal(t, "arccirclegap", gms[6].Name)
	assert.Equal(t, 67, gms[6].Line)
	assert.Equal(t, "filename.cpp", gms[6].Filename)

	assert.Equal(t, " should draw only green", gms[2].Comment)
	assert.Equal(t, " Multiple lines\n should be concatenated\n into a single comment.", gms[4].Comment)
}
