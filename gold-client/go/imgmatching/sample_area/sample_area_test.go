package sample_area

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/golden/go/image/text"
)

func TestMatcher_Match_Failure_InputValidation(t *testing.T) {
	type expectedDebugValues struct {
		sampleAreaWidthTooSmall                   bool
		sampleAreaWidthTooLarge                   bool
		maxDifferentPixelsPerAreaOutOfRange       bool
		sampleAreaChannelDeltaThresholdOutOfRange bool
	}

	test := func(name string, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold int, debugValues expectedDebugValues) {
		t.Run(name, func(t *testing.T) {
			img := text.MustToNRGBA(image8x8AllWhite)
			matcher := Matcher{
				SampleAreaWidth:                 sampleAreaWidth,
				MaxDifferentPixelsPerArea:       maxDifferentPixelsPerArea,
				SampleAreaChannelDeltaThreshold: sampleAreaChannelDeltaThreshold,
			}

			assert.False(t, matcher.Match(img, img))
			assert.Equal(t, matcher.SampleAreaWidthTooSmall(), debugValues.sampleAreaWidthTooSmall)
			assert.Equal(t, matcher.MaxDifferentPixelsPerAreaOutOfRange(), debugValues.maxDifferentPixelsPerAreaOutOfRange)
			assert.Equal(t, matcher.SampleAreaChannelDeltaThresholdOutOfRange(), debugValues.sampleAreaChannelDeltaThresholdOutOfRange)
			assert.Equal(t, matcher.SampleAreaWidthTooLarge(), debugValues.sampleAreaWidthTooLarge)
		})
	}

	test("sample area width too small", 0, 0, 0, expectedDebugValues{sampleAreaWidthTooSmall: true})
	test("max different pixels per area too small", 2, -1, 0, expectedDebugValues{maxDifferentPixelsPerAreaOutOfRange: true})
	test("max different pixels per area too large", 2, 5, 0, expectedDebugValues{maxDifferentPixelsPerAreaOutOfRange: true})
	test("sample area channel delta threshold too small", 2, 0, -1, expectedDebugValues{sampleAreaChannelDeltaThresholdOutOfRange: true})
	test("sample area channel delta threshold too large", 2, 0, 256, expectedDebugValues{sampleAreaChannelDeltaThresholdOutOfRange: true})
	test("sample area width too large for image", 9, 0, 0, expectedDebugValues{sampleAreaWidthTooLarge: true})
}

func TestMatcher_Match_Failure_ImageComparison(t *testing.T) {
	test := func(name, expected, actual string, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold int) {
		t.Run(name, func(t *testing.T) {
			expectedImage := text.MustToNRGBA(expected)
			actualImage := text.MustToNRGBA(actual)
			matcher := Matcher{
				SampleAreaWidth:                 sampleAreaWidth,
				MaxDifferentPixelsPerArea:       maxDifferentPixelsPerArea,
				SampleAreaChannelDeltaThreshold: sampleAreaChannelDeltaThreshold,
			}

			assert.False(t, matcher.Match(expectedImage, actualImage))
			assert.False(t, matcher.SampleAreaWidthTooSmall())
			assert.False(t, matcher.MaxDifferentPixelsPerAreaOutOfRange())
			assert.False(t, matcher.SampleAreaChannelDeltaThresholdOutOfRange())
			assert.False(t, matcher.SampleAreaWidthTooLarge())
		})
	}

	// Test both image orders to help catch issues such as integer under/overflow.
	testBothImageOrders := func(name, expected, actual string, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold int) {
		test(name, expected, actual, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold)
		test(name+"_images_swapped", actual, expected, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold)
	}

	testBothImageOrders("mismatched image sizes", image8x8AllWhite, image7x7AllWhite, 2, 1, 0)
	testBothImageOrders("dense differences", image8x8AllWhite, image8x8WhiteWithDenseBlack, 2, 1, 0)
	testBothImageOrders("dense but minor differences", image8x8AllWhite, image8x8WhiteWithDenseOffWhite, 2, 1, 0)
	testBothImageOrders("dense minor differences, sparse major differences", image8x8AllWhite, image8x8WhiteWithDenseOffWhiteSparseBlack, 2, 1, 0)
}

func TestMatcher_Match_Success(t *testing.T) {

	test := func(name, expected, actual string, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold int) {
		t.Run(name, func(t *testing.T) {
			expectedImage := text.MustToNRGBA(expected)
			actualImage := text.MustToNRGBA(actual)
			matcher := Matcher{
				SampleAreaWidth:                 sampleAreaWidth,
				MaxDifferentPixelsPerArea:       maxDifferentPixelsPerArea,
				SampleAreaChannelDeltaThreshold: sampleAreaChannelDeltaThreshold,
			}

			assert.True(t, matcher.Match(expectedImage, actualImage))
			assert.False(t, matcher.SampleAreaWidthTooSmall())
			assert.False(t, matcher.MaxDifferentPixelsPerAreaOutOfRange())
			assert.False(t, matcher.SampleAreaChannelDeltaThresholdOutOfRange())
			assert.False(t, matcher.SampleAreaWidthTooLarge())
		})
	}

	// Test both image orders to help catch issues such as integer under/overflow.
	testBothImageOrders := func(name, expected, actual string, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold int) {
		test(name, expected, actual, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold)
		test(name+"_images_swapped", actual, expected, sampleAreaWidth, maxDifferentPixelsPerArea, sampleAreaChannelDeltaThreshold)
	}

	testBothImageOrders("identical images", image8x8AllWhite, image8x8AllWhite, 2, 1, 0)
	testBothImageOrders("sparse differences", image8x8AllWhite, image8x8WhiteWithSparseBlack, 2, 1, 0)
	// The same as the "dense differences" failure test, but with different
	// matcher properties.
	testBothImageOrders("dense differences large sample area", image8x8AllWhite, image8x8WhiteWithDenseBlack, 3, 3, 0)
	// The same as the "dense but minor differences" failure test, but with
	// different matcher properties.
	testBothImageOrders("dense but minor differences with tolerance", image8x8AllWhite, image8x8WhiteWithDenseOffWhite, 2, 1, 5)
	// The same as the "dense minor differences, sparse major differences"
	// failure test, but with different matcher properties
	testBothImageOrders("dense minor differences, sparse major differences with tolerance", image8x8AllWhite, image8x8WhiteWithDenseOffWhiteSparseBlack, 2, 1, 5)
}

const image8x8AllWhite = `! SKTEXTSIMPLE
8 8
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`

const image7x7AllWhite = `! SKTEXTSIMPLE
7 7
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`

const image8x8WhiteWithSparseBlack = `! SKTEXTSIMPLE
8 8
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0x00 0xFF 0xFF 0xFF 0x00 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0x00 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0x00
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`

const image8x8WhiteWithDenseBlack = `! SKTEXTSIMPLE
8 8
0xFF 0x00 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0x00 0xFF 0xFF 0x00 0x00 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0x00 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0x00 0x00 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0x00 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`

const image8x8WhiteWithDenseOffWhite = `! SKTEXTSIMPLE
8 8
0xFF 0xFD 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFD 0xFF 0xFF 0xFD 0xFD 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFD 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFD 0xFD 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFD 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`

const image8x8WhiteWithDenseOffWhiteSparseBlack = `! SKTEXTSIMPLE
8 8
0xFF 0xFD 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
0xFF 0x00 0xFF 0xFF 0xFD 0x00 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFD 0xFF 0xFF
0xFF 0xFF 0xFF 0x00 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFD
0xFF 0xFF 0xFF 0xFD 0xFD 0xFF 0xFF 0x00
0xFF 0xFF 0xFF 0xFD 0xFF 0xFF 0xFF 0xFF
0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`
