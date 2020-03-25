// Package text contains an image plain text file format encoder and decoder.
//
// A super simple format of the form:
//
// ! SKTEXTSIMPLE
// width height
// 0x000000ff 0xffffffff ...
// 0xddddddff 0xffffff88 ...
// ...
//
// Where the pixel values are encoded as 0xRRGGBBAA.
//
// Grayscale pixels can be encoded as 0xXX. The two images below are equivalent:
//
// ! SKTEXTSIMPLE
// 2 2
// 0x00 0x11
// 0xaa 0xbb
//
// ! SKTEXTSIMPLE
// 2 2
// 0x000000ff 0x111111ff
// 0xaaaaaaff 0xbbbbbbff
package text

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"strconv"
	"strings"
)

const skTextHeader = "! SKTEXTSIMPLE\n"

// dim returns the dimensions of the image.
func dim(reader *bufio.Reader) (int, int, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, 0, fmt.Errorf("Failed to read header from SKTEXT file: %s", err)
	}
	if line != skTextHeader {
		return 0, 0, fmt.Errorf("Not a valid SKTEXT file: %q %q", line, skTextHeader)
	}
	line, err = reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return 0, 0, fmt.Errorf("Failed to read dimenstions from SKTEXT file: %s", err)
	}
	width := 0
	height := 0
	n, err := fmt.Sscanf(line, "%d %d", &width, &height)
	if err != nil {
		return 0, 0, fmt.Errorf("Not a valid SKTEXT file: %s", err)
	}
	if n != 2 {
		return 0, 0, fmt.Errorf("Not a valid SKTEXT file, couldn't find width and height.")
	}
	return width, height, nil
}

// Decode reads an SKTEXT image from r and returns it as an image.Image.
// The type of Image returned will always be NRGBA.
func Decode(r io.Reader) (image.Image, error) {
	reader := bufio.NewReader(r)
	width, height, err := dim(reader)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode SKTEXT config: %s", err)
	}
	ret := image.NewNRGBA(image.Rect(0, 0, width, height))
	lineNum := 0
	for {
		if lineNum > height {
			return nil, fmt.Errorf("Too many y values: %d > %d", lineNum, height)
		}
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			break
		}
		hexline := strings.Split(line, " ")
		if len(hexline) > width && len(hexline[0]) > 0 {
			return nil, fmt.Errorf("Too many x values: %d > %d", len(hexline), width)
		}
		for i, h := range hexline {
			h = strings.TrimSpace(h)
			if h != "" {
				if !strings.HasPrefix(h, "0x") || (len(h) != 4 && len(h) != 10) {
					return nil, fmt.Errorf("Invalid pixel format, must be 0xRRGGBBAA or 0xXX (for color or grayscale pixels, respectively), got %q", h)
				}
				pixel, err := strconv.ParseUint(strings.TrimSpace(h), 0, 32)
				if err != nil {
					return nil, err
				}
				var r, g, b, a uint8
				if len(h) == 10 {
					// Color notation.
					r = uint8((pixel >> 24) & 0xff)
					g = uint8((pixel >> 16) & 0xff)
					b = uint8((pixel >> 8) & 0xff)
					a = uint8((pixel >> 0) & 0xff)
				} else {
					// Grayscale notation.
					r = uint8(pixel)
					g = uint8(pixel)
					b = uint8(pixel)
					a = uint8(0xff)
				}
				offset := lineNum*ret.Stride + i*4
				ret.Pix[offset+0] = r
				ret.Pix[offset+1] = g
				ret.Pix[offset+2] = b
				ret.Pix[offset+3] = a
			}
		}
		lineNum += 1
		if err != nil {
			break
		}
	}
	if err == nil || err == io.EOF {
		return ret, nil
	}
	return nil, fmt.Errorf("Failed reading SKTEXT file contents: %s", err)
}

// DecodeConfig returns the color model and dimensions of SKTEXT image without
// decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	reader := bufio.NewReader(r)
	width, height, err := dim(reader)
	if err != nil {
		return image.Config{}, fmt.Errorf("Failed to Decode SKTEXT file: %s", err)
	}

	return image.Config{
		ColorModel: color.NRGBAModel,
		Width:      width,
		Height:     height,
	}, nil
}

// Encode encoded the image in SKTEXT format.
func Encode(w io.Writer, m *image.NRGBA) error {
	if _, err := fmt.Fprintf(w, "%s%d %d\n", skTextHeader, m.Rect.Dx(), m.Rect.Dy()); err != nil {
		return err
	}
	height := m.Bounds().Dy()
	for i := 0; i < len(m.Pix); i += 4 {
		_, err := fmt.Fprintf(w, "0x%02x%02x%02x%02x", m.Pix[i+0], m.Pix[i+1], m.Pix[i+2], m.Pix[i+3])
		if err != nil {
			return err
		}
		// Add whitespace.
		if (i > 0 || m.Stride == 4) && (i+4)%m.Stride == 0 {
			// Don't add a trailing \n to the very last line.
			if (i+4)/m.Stride < height {
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			}
		} else {
			if _, err := fmt.Fprint(w, " "); err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	image.RegisterFormat("sktext", skTextHeader, Decode, DecodeConfig)
}

// MustToNRGBA returns an *image.NRGBA from a given string, which is assumed to be an image in the
// SKTEXTSIMPLE "codec". It panics if the string cannot be processed into an image, suitable only
// for testing code.
func MustToNRGBA(s string) *image.NRGBA {
	img, err := Decode(strings.NewReader(s))
	if err != nil {
		// This indicates an error with the static test data.
		panic(fmt.Sprintf("Failed to decode a valid image: %s", err))
	}
	return img.(*image.NRGBA)
}

// MustToGray calls MustToNRGBA with the given string and converts the returned image to grayscale.
func MustToGray(s string) *image.Gray {
	nrgba := MustToNRGBA(s)
	gray := image.NewGray(nrgba.Bounds())
	draw.Draw(gray, nrgba.Bounds(), nrgba, nrgba.Bounds().Min, draw.Src)
	return gray
}
