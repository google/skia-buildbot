// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package butlerproto

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/data/recordio"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
)

const (
	// DefaultCompressThreshold is the byte size threshold for compressing message
	// data. Messages whose byte count is less than or equal to this threshold
	// will not be compressed.
	//
	// This is the value used by Akamai for its compression threshold:
	// "The reasons 860 bytes is the minimum size for compression is twofold:
	// (1) The overhead of compressing an object under 860 bytes outweighs
	// performance gain. (2) Objects under 860 bytes can be transmitted via a
	// single packet anyway, so there isn't a compelling reason to compress them."
	DefaultCompressThreshold = 860
)

// protoBase is the base type of protocol reader/writer objects.
type protoBase struct {
	// maxSize is the maximum Butler protocol data size. By default, it is
	// types.MaxButlerLogBundleSize. However, it can be overridden for testing
	// here.
	maxSize int64
}

func (p *protoBase) getMaxSize() int64 {
	if p.maxSize == 0 {
		return types.MaxButlerLogBundleSize
	}
	return p.maxSize
}

// Reader is a protocol reader instance.
type Reader struct {
	protoBase

	// Metadata is the unpacked ButlerMetadata. It is populated when the
	// metadata has been read.
	Metadata *logpb.ButlerMetadata

	// Bundle is the unpacked ButlerLogBundle. It is populated when the
	// protocol data has been read and the Metadata indicates a ButlerLogBundle
	// type.
	Bundle *logpb.ButlerLogBundle
}

// ReadMetadata reads the metadata header frame.
func (r *Reader) readMetadata(fr recordio.Reader) error {
	data, err := fr.ReadFrameAll()
	if err != nil {
		return err
	}

	md := logpb.ButlerMetadata{}
	if err := proto.Unmarshal(data, &md); err != nil {
		return fmt.Errorf("butlerproto: failed to unmarshal Metadata frame: %s", err)
	}
	r.Metadata = &md
	return nil
}

func (r *Reader) readData(fr recordio.Reader) ([]byte, error) {
	size, br, err := fr.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle frame: %s", err)
	}

	// Read the frame through a zlib reader.
	switch r.Metadata.Compression {
	case logpb.ButlerMetadata_NONE:
		break

	case logpb.ButlerMetadata_ZLIB:
		br, err = zlib.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize zlib reader: %s", err)
		}

	default:
		return nil, fmt.Errorf("unknown compression type: %v", r.Metadata.Compression)
	}

	// Wrap our reader in a limitErrorReader so we don't pull data beyond our
	// soft maximum.
	br = &limitErrorReader{
		Reader: br,
		limit:  r.getMaxSize(),
	}

	buf := bytes.Buffer{}
	buf.Grow(int(size))
	_, err = buf.ReadFrom(br)
	if err != nil {
		return nil, fmt.Errorf("butlerproto: failed to buffer bundle frame: %s", err)
	}
	return buf.Bytes(), nil
}

func (r *Reader) Read(ir io.Reader) error {
	fr := recordio.NewReader(ir, r.getMaxSize())

	// Ensure that we have our Metadata.
	if err := r.readMetadata(fr); err != nil {
		return err
	}

	switch r.Metadata.Type {
	case logpb.ButlerMetadata_ButlerLogBundle:
		data, err := r.readData(fr)
		if err != nil {
			return fmt.Errorf("butlerproto: failed to read Bundle data: %s", err)
		}

		if r.Metadata.ProtoVersion == logpb.Version {
			bundle := logpb.ButlerLogBundle{}
			if err := proto.Unmarshal(data, &bundle); err != nil {
				return fmt.Errorf("butlerproto: failed to unmarshal Bundle frame: %s", err)
			}
			r.Bundle = &bundle
		}
		return nil

	default:
		return fmt.Errorf("butlerproto: unknown data type: %s", r.Metadata.Type)
	}
}

// limitErrorReader is similar to io.LimitReader, except that it returns
// a custom error instead of io.EOF.
//
// This is important, as it allows us to distinguish between the end of
// the compressed reader's data and a limit being hit.
type limitErrorReader struct {
	io.Reader       // underlying reader
	limit     int64 // max bytes remaining
}

func (r *limitErrorReader) Read(p []byte) (int, error) {
	if r.limit <= 0 {
		return 0, errors.New("limit exceeded")
	}
	if int64(len(p)) > r.limit {
		p = p[0:r.limit]
	}
	n, err := r.Reader.Read(p)
	r.limit -= int64(n)
	return n, err
}

// Writer writes Butler messages that the Reader can read.
type Writer struct {
	protoBase

	// ProtoVersion is the protocol version string to use. If empty, the current
	// ProtoVersion will be used.
	ProtoVersion string

	// Compress, if true, allows the Writer to choose to compress data when
	// applicable.
	Compress bool

	// CompressThreshold is the minimum size that data must be in order to
	CompressThreshold int

	compressBuf    bytes.Buffer
	compressWriter *zlib.Writer
}

func (w *Writer) writeData(fw recordio.Writer, t logpb.ButlerMetadata_ContentType, data []byte) error {
	if int64(len(data)) > w.getMaxSize() {
		return fmt.Errorf("butlerproto: serialized size exceeds soft cap (%d > %d)", len(data), w.getMaxSize())
	}

	pv := w.ProtoVersion
	if pv == "" {
		pv = logpb.Version
	}
	md := logpb.ButlerMetadata{
		Type:         t,
		ProtoVersion: pv,
	}

	// If we're configured to compress and the data is below our threshold,
	// compress.
	if w.Compress && len(data) >= w.CompressThreshold {
		w.compressBuf.Reset()
		if w.compressWriter == nil {
			w.compressWriter = zlib.NewWriter(&w.compressBuf)
		} else {
			w.compressWriter.Reset(&w.compressBuf)
		}
		if _, err := w.compressWriter.Write(data); err != nil {
			return err
		}
		if err := w.compressWriter.Close(); err != nil {
			return err
		}

		compressed := true
		if compressed {
			md.Compression = logpb.ButlerMetadata_ZLIB
		}
		data = w.compressBuf.Bytes()
	}

	// Write metadata frame.
	mdData, err := proto.Marshal(&md)
	if err != nil {
		return fmt.Errorf("butlerproto: failed to marshal Metadata: %s", err)
	}
	_, err = fw.Write(mdData)
	if err != nil {
		return fmt.Errorf("butlerproto: failed to write Metadata frame: %s", err)
	}
	if err := fw.Flush(); err != nil {
		return fmt.Errorf("butlerproto: failed to flush Metadata frame: %s", err)
	}

	// Write data frame.
	_, err = fw.Write(data)
	if err != nil {
		return fmt.Errorf("butlerproto: failed to write data frame: %s", err)
	}
	if err := fw.Flush(); err != nil {
		return fmt.Errorf("butlerproto: failed to flush data frame: %s", err)
	}
	return nil
}

// WriteWith writes a ButlerLogBundle to the supplied Writer.
func (w *Writer) Write(iw io.Writer, b *logpb.ButlerLogBundle) error {
	return w.WriteWith(recordio.NewWriter(iw), b)
}

// WriteWith writes a ButlerLogBundle to the supplied recordio.Writer.
func (w *Writer) WriteWith(fw recordio.Writer, b *logpb.ButlerLogBundle) error {
	data, err := proto.Marshal(b)
	if err != nil {
		return fmt.Errorf("butlerproto: failed to marshal Bundle: %s", b)
	}

	return w.writeData(fw, logpb.ButlerMetadata_ButlerLogBundle, data)
}
