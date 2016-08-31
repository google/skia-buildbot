// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package signing

import (
	"golang.org/x/net/context"
)

// Signer holds private key and corresponding cert and can sign blobs.
//
// It signs using RSA-256 + PKCS1v15. It usually lives in a context, see
// GetSigner function.
type Signer interface {
	// SignBytes signs the blob with some active private key.
	//
	// Hashes the blob using SHA256 and then calculates RSASSA-PKCS1-v1_5
	// signature using the currently active signing key.
	//
	// Returns the signature and name of the key used.
	SignBytes(c context.Context, blob []byte) (keyName string, signature []byte, err error)

	// Certificates returns a bundle with public certificates for all active keys.
	//
	// The certificates contains public keys that can be used to validate
	// signatures generated with SignBytes. See CheckSignature method of
	// PublicCertificates.
	//
	// Do not modify return value, it may be shared by many callers.
	Certificates(c context.Context) (*PublicCertificates, error)

	// ServiceInfo returns information about the current service.
	//
	// It includes app ID and the service account name (that ultimately owns the
	// signing private key).
	//
	// Do not modify return value, it may be shared by many callers.
	ServiceInfo(c context.Context) (*ServiceInfo, error)
}
