// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamproto

var (
	// ProtocolFrameHeaderMagic is the number at the beginning of streams that
	// identifies the stream handshake version.
	//
	// This serves two purposes:
	//   - To disambiguate a Butler stream from some happenstance string of bytes
	//     (which probably won't start with these characters).
	//   - To allow an upgrade to the wire format, if one is ever needed. e.g.,
	//     a switch to something other than recordio/JSON.
	ProtocolFrameHeaderMagic = []byte("BTLR1\x1E")
)
