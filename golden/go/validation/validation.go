package validation

import (
	"strings"

	"go.skia.org/infra/golden/go/diffstore/common"
)

// IsValidDigest returns true if the given string is a valid digest
// on the string level, i.e. it does not check whether we have
// actually seen the given hash but whether it complies with the format
// that we expect for a hash.
func IsValidDigest(hash string) bool {
	// Currently we expect all digests to be ASCII encoded MD5 hashes.
	if len(hash) != 32 {
		return false
	}

	for _, c := range []byte(hash) {
		if ((c >= '0') && (c <= '9')) ||
			((c >= 'a') && (c <= 'f')) ||
			((c >= 'A') && (c <= 'F')) {
			continue
		}
		return false
	}
	return true
}

// IsValidDiffImgID returns true if the given diffImgID is in the correct format.
func IsValidDiffImgID(diffID string) bool {
	imageIDs := strings.Split(diffID, common.DiffImageSeparator)
	if len(imageIDs) != 2 {
		return false
	}
	return IsValidDigest(imageIDs[0]) && IsValidDigest(imageIDs[1])
}
