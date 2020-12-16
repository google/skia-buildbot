package sql

import (
	"crypto/md5"
	"encoding/hex"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

// DigestToBytes returns the given digest as bytes. It returns an error if a missing digest is
// passed in or if the bytes are otherwise invalid.
func DigestToBytes(d types.Digest) (schema.Digest, error) {
	if len(d) != 32 {
		return schema.Digest{}, skerr.Fmt("Empty/missing/invalid digest passed in %q", d)
	}
	out := make(schema.Digest, md5.Size)
	_, err := hex.Decode(out, []byte(d))
	return out, skerr.Wrap(err)
}
