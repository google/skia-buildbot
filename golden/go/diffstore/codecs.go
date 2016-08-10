package diffstore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
)

// NRGBACodec serializes and deserializes NRGBA images.
type NRGBACodec struct{}

// Encode implements util.LRUCodec.
func (n NRGBACodec) Encode(data interface{}) ([]byte, error) {
	img := data.(*image.NRGBA)
	buf := bytes.NewBuffer(make([]byte, 0, len(img.Pix)+24))

	if err := binary.Write(buf, binary.LittleEndian, int64(len(img.Pix))); err != nil {
		return nil, err
	}

	if _, err := buf.Write(img.Pix); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, int64(img.Rect.Dx())); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, int64(img.Rect.Dy())); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decode implements util.LRUCodec.
func (n NRGBACodec) Decode(byteData []byte) (interface{}, error) {
	buf := bytes.NewBuffer(byteData)

	var imgLen int64
	if err := binary.Read(buf, binary.LittleEndian, &imgLen); err != nil {
		return nil, err
	}
	// fmt.Printf("Decode len: %d\n", imgLen)

	pix := make([]byte, imgLen)
	if n, err := buf.Read(pix); (err != nil) || (int64(n) != imgLen) {
		return nil, fmt.Errorf("Expected to read %d bytes. Got %d bytes and error: %v", imgLen, n, err)
	}
	// fmt.Printf("Decode read: %d\n", len(pix))

	var width, height int64
	if err := binary.Read(buf, binary.LittleEndian, &width); err != nil {
		return nil, err
	}

	if err := binary.Read(buf, binary.LittleEndian, &height); err != nil {
		return nil, err
	}

	rect := image.Rect(0, 0, int(width), int(height))
	ret := &image.NRGBA{
		Pix:    pix,
		Stride: int(4 * width),
		Rect:   rect,
	}
	return ret, nil
}
