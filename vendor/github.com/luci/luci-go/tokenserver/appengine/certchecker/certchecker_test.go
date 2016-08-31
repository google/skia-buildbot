// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package certchecker

import (
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/appengine/gaetesting"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/data/rand/cryptorand"
	"github.com/luci/luci-go/tokenserver/appengine/model"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCertChecker(t *testing.T) {
	Convey("CertChecker works", t, func() {
		ctx := gaetesting.TestingContext()
		ctx = cryptorand.MockForTest(ctx, 0)
		ctx, clk := testclock.UseTime(ctx, testclock.TestTimeUTC)

		// Generate new CA private key and certificate.
		pkey, caCert, err := generateCA(ctx, "Some CA: ca-name.fake")
		So(err, ShouldBeNil)

		// Nothing in the datastore yet.
		checker, err := GetCertChecker(ctx, "Some CA: ca-name.fake")
		So(err, ShouldNotBeNil)

		// Put it into the datastore.
		caEntity := model.CA{
			CN:    "Some CA: ca-name.fake",
			Cert:  caCert,
			Ready: true,
		}
		err = datastore.Get(ctx).Put(&caEntity)
		So(err, ShouldBeNil)

		// In the datastore now.
		checker, err = GetCertChecker(ctx, "Some CA: ca-name.fake")
		So(err, ShouldBeNil)

		// Update associated CRL (it's empty).
		err = model.UpdateCRLSet(ctx, "Some CA: ca-name.fake", model.CRLShardCount, &pkix.CertificateList{})
		So(err, ShouldBeNil)

		// Generate some certificate signed by the CA.
		certDer, err := generateCert(ctx, 2, "some-cert-name.fake", caCert, pkey)
		So(err, ShouldBeNil)

		// Use CertChecker to check its validity. Need to parse DER first.
		parsedCert, err := x509.ParseCertificate(certDer)
		So(err, ShouldBeNil)
		So(parsedCert.Issuer.CommonName, ShouldEqual, "Some CA: ca-name.fake")

		// Valid!
		ca, err := checker.CheckCertificate(ctx, parsedCert)
		So(err, ShouldBeNil)
		So(ca.CN, ShouldEqual, "Some CA: ca-name.fake")
		So(ca.ParsedConfig, ShouldNotBeNil)

		// Revoke the certificate by generating new CRL and putting it into the
		// datastore.
		err = model.UpdateCRLSet(ctx, "Some CA: ca-name.fake", model.CRLShardCount, &pkix.CertificateList{
			TBSCertList: pkix.TBSCertificateList{
				RevokedCertificates: []pkix.RevokedCertificate{
					{SerialNumber: big.NewInt(2)},
				},
			},
		})
		So(err, ShouldBeNil)

		// Bump time to invalidate cert checker caches.
		clk.Add(10 * time.Minute)

		// Check same cert again. Should be rejected now as revoked.
		_, err = checker.CheckCertificate(ctx, parsedCert)
		So(err, ShouldErrLike, "certificate with SN 2 has been revoked")

		// Fast forward past cert expiration time.
		clk.Add(6 * time.Hour)

		// Should be rejected as expired now.
		_, err = checker.CheckCertificate(ctx, parsedCert)
		So(err, ShouldErrLike, "certificate has expired")

		// Generate some cert with wrong signature (use different private key).
		phonyCAKey, err := rsa.GenerateKey(cryptorand.Get(ctx), 512)
		So(err, ShouldBeNil)
		certDer, err = generateCert(ctx, 3, "some-name", caCert, phonyCAKey)
		So(err, ShouldBeNil)

		// CertChecker rejects it.
		parsedCert, _ = x509.ParseCertificate(certDer)
		_, err = checker.CheckCertificate(ctx, parsedCert)
		So(err, ShouldErrLike, "crypto/rsa: verification error")
	})
}

func generateCA(c context.Context, name string) (*rsa.PrivateKey, []byte, error) {
	// See https://golang.org/src/crypto/tls/generate_cert.go.
	rand := cryptorand.Get(c)
	privKey, err := rsa.GenerateKey(rand, 512) // use short key in tests
	if err != nil {
		return nil, nil, err
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             clock.Now(c),
		NotAfter:              clock.Now(c).Add(30 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand, &template, &template, privKey.Public(), privKey)
	if err != nil {
		return nil, nil, err
	}
	return privKey, derBytes, nil
}

func generateCert(c context.Context, sn int64, name string, caCert []byte, caKey *rsa.PrivateKey) ([]byte, error) {
	parent, err := x509.ParseCertificate(caCert)
	if err != nil {
		return nil, nil
	}
	rand := cryptorand.Get(c)
	privKey, err := rsa.GenerateKey(rand, 512) // use short key in tests
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(sn),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             clock.Now(c),
		NotAfter:              clock.Now(c).Add(5 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	return x509.CreateCertificate(rand, &template, parent, privKey.Public(), caKey)
}
