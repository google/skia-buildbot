// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tokenminter

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/appengine/gaetesting"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/server/auth/signing/signingtest"

	"github.com/luci/luci-go/tokenserver/api"
	"github.com/luci/luci-go/tokenserver/api/admin/v1"
	"github.com/luci/luci-go/tokenserver/api/minter/v1"

	"github.com/luci/luci-go/tokenserver/appengine/certchecker"
	"github.com/luci/luci-go/tokenserver/appengine/model"
	"github.com/luci/luci-go/tokenserver/appengine/services/admin/serviceaccounts"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMintOAuthToken(t *testing.T) {
	Convey("with mock context", t, func() {
		ctx := gaetesting.TestingContext()
		ctx, _ = testclock.UseTime(ctx, time.Date(2015, time.February, 3, 4, 5, 6, 7, time.UTC))

		Convey("success", func(c C) {
			server := makeTestServer(&Server{
				mintOAuthToken: func(_ context.Context, p serviceaccounts.MintAccessTokenParams) (*tokenserver.ServiceAccount, *minter.OAuth2AccessToken, error) {
					c.So(p.FQDN, ShouldEqual, "luci-token-server-test-1.fake.domain")
					c.So(p.Scopes, ShouldResemble, []string{"scope2", "scope1"})
					sa := &tokenserver.ServiceAccount{Email: "blah@email.com"}
					tok := &minter.OAuth2AccessToken{AccessToken: "access-token"}
					return sa, tok, nil
				},
				certChecker: func(_ context.Context, cert *x509.Certificate) (*model.CA, error) {
					c.So(cert.Subject.CommonName, ShouldEqual, "luci-token-server-test-1.fake.domain")
					c.So(cert.Issuer.CommonName, ShouldEqual, "Fake CA: fake.ca")
					return &model.CA{
						CN: cert.Issuer.CommonName,
						ParsedConfig: &admin.CertificateAuthorityConfig{
							KnownDomains: []*admin.DomainConfig{
								{
									Domain:             []string{"fake.domain"},
									AllowedOauth2Scope: []string{"scope1", "scope2"},
								},
							},
						},
					}, nil
				},
			})

			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
				Oauth2Scopes:       []string{"scope2", "scope1"},
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ServiceVersion: "app/testVersionID",
				TokenResponse: &minter.MachineTokenResponse{
					ServiceAccount: &tokenserver.ServiceAccount{Email: "blah@email.com"},
					ServiceVersion: "app/testVersionID",
					TokenType: &minter.MachineTokenResponse_GoogleOauth2AccessToken{
						GoogleOauth2AccessToken: &minter.OAuth2AccessToken{AccessToken: "access-token"},
					},
				},
			})
		})

		Convey("broken serialization", func(c C) {
			server := makeTestServer(&Server{})
			_, err := server.MintMachineToken(ctx, &minter.MintMachineTokenRequest{
				SerializedTokenRequest: []byte("Im not a proto"),
			})
			So(grpc.Code(err), ShouldEqual, codes.InvalidArgument)
		})

		Convey("unsupported token kind", func(c C) {
			server := makeTestServer(&Server{})
			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
				TokenType:          1234, // unsupported
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_UNSUPPORTED_TOKEN_TYPE,
				ErrorMessage:   "token_type 1234 is not supported",
				ServiceVersion: "app/testVersionID",
			})
		})

		Convey("no timestamp", func(c C) {
			server := makeTestServer(&Server{})
			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_BAD_TIMESTAMP,
				ErrorMessage:   "issued_at is required",
				ServiceVersion: "app/testVersionID",
			})
		})

		Convey("timestamp too old", func(c C) {
			server := makeTestServer(&Server{})
			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx).Add(-11 * time.Minute)),
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_BAD_TIMESTAMP,
				ErrorMessage:   "issued_at timestamp is not within acceptable range, check your clock",
				ServiceVersion: "app/testVersionID",
			})
		})

		Convey("timestamp too new", func(c C) {
			server := makeTestServer(&Server{})
			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx).Add(11 * time.Minute)),
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_BAD_TIMESTAMP,
				ErrorMessage:   "issued_at timestamp is not within acceptable range, check your clock",
				ServiceVersion: "app/testVersionID",
			})
		})

		Convey("malformed certificate", func(c C) {
			server := makeTestServer(&Server{})
			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        []byte("Im not a certificate"),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_BAD_CERTIFICATE_FORMAT,
				ErrorMessage:   "failed to parse the certificate (expecting x509 cert DER)",
				ServiceVersion: "app/testVersionID",
			})
		})

		Convey("unsupported signature algo", func(c C) {
			server := makeTestServer(&Server{})
			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: 1234, // unsupported
				IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_UNSUPPORTED_SIGNATURE,
				ErrorMessage:   "signature_algorithm 1234 is not supported",
				ServiceVersion: "app/testVersionID",
			})
		})

		Convey("broken signature", func(c C) {
			server := makeTestServer(&Server{})

			req := makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			})
			So(req.Signature[0], ShouldNotEqual, 0)
			req.Signature[0] = 0 // break it

			resp, err := server.MintMachineToken(ctx, req)
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_BAD_SIGNATURE,
				ErrorMessage:   "signature verification failed - crypto/rsa: verification error",
				ServiceVersion: "app/testVersionID",
			})
		})

		Convey("revoked cert", func(c C) {
			server := makeTestServer(&Server{
				certChecker: func(_ context.Context, cert *x509.Certificate) (*model.CA, error) {
					return nil, certchecker.NewError(fmt.Errorf("revoked cert"), certchecker.CertificateRevoked)
				},
			})
			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
				Oauth2Scopes:       []string{"scope2", "scope1"},
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_UNTRUSTED_CERTIFICATE,
				ErrorMessage:   "revoked cert",
				ServiceVersion: "app/testVersionID",
			})
		})

		Convey("unknown domain", func(c C) {
			server := makeTestServer(&Server{
				certChecker: func(_ context.Context, cert *x509.Certificate) (*model.CA, error) {
					c.So(cert.Subject.CommonName, ShouldEqual, "luci-token-server-test-1.fake.domain")
					return &model.CA{
						CN: "Fake CA: fake.ca",
						ParsedConfig: &admin.CertificateAuthorityConfig{
							KnownDomains: []*admin.DomainConfig{
								{
									Domain:             []string{"something-other-than-fake.domain"},
									AllowedOauth2Scope: []string{"scope1", "scope2"},
								},
							},
						},
					}, nil
				},
			})

			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
				TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
				Oauth2Scopes:       []string{"scope2", "scope1"},
			}))
			So(err, ShouldBeNil)
			So(resp, ShouldResemble, &minter.MintMachineTokenResponse{
				ErrorCode:      minter.ErrorCode_BAD_TOKEN_ARGUMENTS,
				ErrorMessage:   "the domain \"fake.domain\" is not whitelisted in the config",
				ServiceVersion: "app/testVersionID",
			})
		})

	})
}

func TestLuciMachineToken(t *testing.T) {
	Convey("with mock context and server", t, func() {
		ctx := gaetesting.TestingContext()
		ctx, tc := testclock.UseTime(ctx, time.Date(2015, time.February, 3, 4, 5, 6, 7, time.UTC))

		server := makeTestServer(&Server{
			certChecker: func(_ context.Context, cert *x509.Certificate) (*model.CA, error) {
				return &model.CA{
					CN: "Fake CA: fake.ca",
					ParsedConfig: &admin.CertificateAuthorityConfig{
						UniqueId: 123,
						KnownDomains: []*admin.DomainConfig{
							{
								Domain:               []string{"fake.domain"},
								MachineTokenLifetime: 3600,
							},
						},
					},
				}, nil
			},
			signer:  signingtest.NewSigner(0, nil),
			isAdmin: func(context.Context) (bool, error) { return true, nil },
		})

		// Put CA and CRL config in the datastore.
		ca := model.CA{
			CN:    "Fake CA: fake.ca",
			Ready: true,
		}
		datastore.Get(ctx).Put(&ca)
		model.StoreCAUniqueIDToCNMap(ctx, map[int64]string{
			123: "Fake CA: fake.ca",
		})
		model.UpdateCRLSet(ctx, "Fake CA: fake.ca", model.CRLShardCount, &pkix.CertificateList{})

		Convey("success", func(c C) {
			resp, err := server.MintMachineToken(ctx, makeTestRequest(&minter.MachineTokenRequest{
				Certificate:        getTestCertDER(),
				SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
				IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
				TokenType:          minter.TokenType_LUCI_MACHINE_TOKEN,
			}))
			So(err, ShouldBeNil)

			tok := resp.TokenResponse.GetLuciMachineToken().MachineToken
			So(tok, ShouldEqual, `CkkKJGx1Y2ktdG9rZW4tc2VydmVyLXRlc3QtMS5mYWtlLmRvbW`+
				`FpbhITc2lnbmVyQHRlc3RpbmcuaG9zdBjykcGmBSCQHCh7MIAgEihmOWRhNWEwZDA5MDN`+
				`iZGE1OGM2ZDY2NGUzODUyYTg5YzI4M2Q3ZmU5GkCrjVagpGYIIVo/OsbQD4Jg8Jbp1l/6`+
				`Q+9t+jHpBXcennnPVb0tKWojPYN3195lZ1ESofXz8rDnXoYF4/2X2pDn`)

			// Works!
			reply, err := server.InspectMachineToken(ctx, &minter.InspectMachineTokenRequest{
				TokenType: minter.TokenType_LUCI_MACHINE_TOKEN,
				Token:     tok,
			})
			So(err, ShouldBeNil)
			So(reply, ShouldResemble, &minter.InspectMachineTokenResponse{
				Valid:        true,
				Signed:       true,
				NonExpired:   true,
				NonRevoked:   true,
				SigningKeyId: "f9da5a0d0903bda58c6d664e3852a89c283d7fe9",
				CertCaName:   "Fake CA: fake.ca",
				TokenType: &minter.InspectMachineTokenResponse_LuciMachineToken{
					LuciMachineToken: &tokenserver.MachineTokenBody{
						MachineFqdn: "luci-token-server-test-1.fake.domain",
						IssuedBy:    "signer@testing.host",
						IssuedAt:    1422936306,
						Lifetime:    3600,
						CaId:        123,
						CertSn:      4096,
					},
				},
			})

			// "Break" signature.
			reply, err = server.InspectMachineToken(ctx, &minter.InspectMachineTokenRequest{
				TokenType: minter.TokenType_LUCI_MACHINE_TOKEN,
				Token:     tok[:len(tok)-11] + "0" + tok[len(tok)-10:],
			})
			So(err, ShouldBeNil)
			So(reply, ShouldResemble, &minter.InspectMachineTokenResponse{
				Valid:            false,
				InvalidityReason: "can't validate signature - crypto/rsa: verification error",
				Signed:           false,
				NonExpired:       false,
				NonRevoked:       false,
				SigningKeyId:     "f9da5a0d0903bda58c6d664e3852a89c283d7fe9",
				TokenType: &minter.InspectMachineTokenResponse_LuciMachineToken{
					LuciMachineToken: &tokenserver.MachineTokenBody{
						MachineFqdn: "luci-token-server-test-1.fake.domain",
						IssuedBy:    "signer@testing.host",
						IssuedAt:    1422936306,
						Lifetime:    3600,
						CaId:        123,
						CertSn:      4096,
					},
				},
			})

			// Expired.
			tc.Add(time.Hour + 11*time.Minute)
			reply, err = server.InspectMachineToken(ctx, &minter.InspectMachineTokenRequest{
				TokenType: minter.TokenType_LUCI_MACHINE_TOKEN,
				Token:     tok,
			})
			So(err, ShouldBeNil)
			So(reply, ShouldResemble, &minter.InspectMachineTokenResponse{
				Valid:            false,
				InvalidityReason: "expired",
				Signed:           true,
				NonExpired:       false,
				NonRevoked:       true,
				SigningKeyId:     "f9da5a0d0903bda58c6d664e3852a89c283d7fe9",
				CertCaName:       "Fake CA: fake.ca",
				TokenType: &minter.InspectMachineTokenResponse_LuciMachineToken{
					LuciMachineToken: &tokenserver.MachineTokenBody{
						MachineFqdn: "luci-token-server-test-1.fake.domain",
						IssuedBy:    "signer@testing.host",
						IssuedAt:    1422936306,
						Lifetime:    3600,
						CaId:        123,
						CertSn:      4096,
					},
				},
			})

			// "Revoke" the certificate. Bump time to expire caches. Note that the
			// token is already expired too.
			model.UpdateCRLSet(ctx, "Fake CA: fake.ca", model.CRLShardCount, &pkix.CertificateList{
				TBSCertList: pkix.TBSCertificateList{
					RevokedCertificates: []pkix.RevokedCertificate{
						{SerialNumber: big.NewInt(4096)},
					},
				},
			})
			tc.Add(5 * time.Minute)
			reply, err = server.InspectMachineToken(ctx, &minter.InspectMachineTokenRequest{
				TokenType: minter.TokenType_LUCI_MACHINE_TOKEN,
				Token:     tok,
			})
			So(err, ShouldBeNil)
			So(reply, ShouldResemble, &minter.InspectMachineTokenResponse{
				Valid:            false,
				InvalidityReason: "expired", // "expired" 'beats' revocation
				Signed:           true,
				NonExpired:       false,
				NonRevoked:       false, // revoked now!
				SigningKeyId:     "f9da5a0d0903bda58c6d664e3852a89c283d7fe9",
				CertCaName:       "Fake CA: fake.ca",
				TokenType: &minter.InspectMachineTokenResponse_LuciMachineToken{
					LuciMachineToken: &tokenserver.MachineTokenBody{
						MachineFqdn: "luci-token-server-test-1.fake.domain",
						IssuedBy:    "signer@testing.host",
						IssuedAt:    1422936306,
						Lifetime:    3600,
						CaId:        123,
						CertSn:      4096,
					},
				},
			})

		})
	})
}

////

func makeTestServer(s *Server) *Server {
	if s.mintOAuthToken == nil {
		s.mintOAuthToken = func(context.Context, serviceaccounts.MintAccessTokenParams) (*tokenserver.ServiceAccount, *minter.OAuth2AccessToken, error) {
			panic("must not be called")
		}
	}

	if s.certChecker == nil {
		s.certChecker = func(context.Context, *x509.Certificate) (*model.CA, error) {
			panic("must not be called")
		}
	}

	if s.isAdmin == nil {
		s.isAdmin = func(context.Context) (bool, error) {
			panic("must not be called")
		}
	}

	// This is called in almost all test cases, make it the default.
	if s.signerServiceAccount == nil {
		s.signerServiceAccount = func(context.Context) (string, error) {
			return "signer@testing.host", nil
		}
	}

	return s
}

func makeTestRequest(req *minter.MachineTokenRequest) *minter.MintMachineTokenRequest {
	serialized, err := proto.Marshal(req)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(serialized)
	signature, err := getTestPrivateKey().Sign(nil, digest[:], crypto.SHA256)
	if err != nil {
		panic(err)
	}
	return &minter.MintMachineTokenRequest{
		SerializedTokenRequest: serialized,
		Signature:              signature,
	}
}

func getTestPrivateKey() *rsa.PrivateKey {
	block, _ := pem.Decode([]byte(pkey))
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	return key
}

func getTestCert() *x509.Certificate {
	crt, err := x509.ParseCertificate(getTestCertDER())
	if err != nil {
		panic(err)
	}
	return crt
}

func getTestCertDER() []byte {
	block, _ := pem.Decode([]byte(cert))
	return block.Bytes
}

const pkey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAvc6v42I4badqYA+IF9dMB838Q2l2IflSpA8xSC5O7XrDwa1R
YCqPq+MOIIaMUgqBMJz0OmNyQkbtRLq3Qu4Q44UIbdqyy34rj7kcw/9t/K9x+2ne
Phx0tfdz+5Lj6UiRRI7FRCi9cs+mgSQquCDaBW8J5l3lCZEne8fpHPO3Hxl+dkUX
0Y8T2ZCsn19hnV7Z7wbfN1dUuRihXD+UwN2axoqZ0EJ2GNSLYAz3HHkKc6ELM1Lx
biOD9Jxw8wb+5VnpIuR3l426Fcux9EQGewLZFTxjRS7DRPL/9L0xE+yRJ/I04UyJ
v4Ws98fYp/vAM922Wt21P9Py6vgn+Xjyz2AoyQIDAQABAoIBABKQhq+Mycwf1c2z
dzItwqf4w7WsOPu1sRmOytkuflXH7iGhXBY103uSZ61Su6LCmEQy9chINcK5wTc5
s/b95fT67Aoim94/Zu9VwbSB5TYTyug2QKB+lAPAQj3W7ifBC0RTWoQCIBV8reJv
sSX1QJ3LcIJxqJc49U2sDebhB4YpAv7xmy4IfpqX+0iURtXrgBmp0hWKTQb7kRdG
BycDU9/AadgkI1PEhRdbfJ4VHFKxkeSRwPyp1UKvzydfe7Nw2HWlflEH4fZCc29x
AM52K5zN/7ns/xEz9XPOUG0/pBcXeQNA5rbTGoZrhQda/aBWbI9TYGWh7XZFvx5y
vZ/xlckCgYEA6ULnJYn+DDUfa1eKwYsYvzf82pbfBvrpTsIKi9h0CWy7GltYuSJk
6yt6pzEmAeV+eY6/F8pPxN0TTZHQAVcRHiMbazmLiaLUeuZCvIZwU54ttyENC2+k
fLUlt3a5eiPKBZEPGx++HuESWVY0LYk8hcg9koc4+AIsiifXz9kgzRcCgYEA0E9h
Dn1qWEXYxzV6Efcux62d9P8vwRx+ZWtbCdMleosVAWQ4RtS8HKfjYvJdPcKJlaGO
b7VyYbJzayPs0skhayEYXajDhwcykxNCYJTXxSqh3Hf4yEeRLquDLW468a9tRc8q
Q2wv+lav7ZeW+Db35fq0mEHRaUn0iXFiq9c1JR8CgYEA16ocrk98TGsdRpCk4Lcr
RTiNlsihIgIAjenH+G5DMqeOAhts15beObR0bXp6ioxVuCvrsCJESF6iRzjGWUbX
s8Z/xk5pHfMngw27rDScTCNWXxe2yNkK+qY9XffuGuhWE3l/vvNFQ6WS4nhaO7PD
+mkdzIkredoAtieKWEiHFDcCgYBQetqcpoe3owSlslt/JWjFbKZiSVVB3qhWtqtt
mE4akjGDYBz+AKLMz3BighDUE5zkWo6VShzu8er1seOFbH+kzByF0vX37Sf0+rPi
bJ8QZfAzJYbQmhXVWh5MJxJO3d/x4KALfHjs1yERQkfpjhMonzu2t3cYnqIDl/Lv
QS4fMQKBgFx5masOJqHNx/HDOLtOvO8MeKbZgb2wzrBteJBs/KyFjP4fzZZseiVV
67XuwVxrLup7KzUaHK8PysA+ZgiT4ZlvyX+J+pFZA2XPtKTKCA3bKYtIG2JF5W1v
uHXl2FV53+kI2rF188v3jbuUhK0FrsUEXpN8C+dotMMLCLakbNXP
-----END RSA PRIVATE KEY-----`

// Cert for luci-token-server-test-1.fake.domain. Signed by 'Fake CA: fake.ca'.
const cert = `-----BEGIN CERTIFICATE-----
MIIEFjCCAv6gAwIBAgICEAAwDQYJKoZIhvcNAQELBQAwYDELMAkGA1UEBhMCVVMx
EzARBgNVBAgMCkNhbGlmb3JuaWExDTALBgNVBAcMBEJsYWgxEjAQBgNVBAoMCVN0
dWZmIEluYzEZMBcGA1UEAwwQRmFrZSBDQTogZmFrZS5jYTAeFw0xNjA0MDkwNDIx
MTJaFw0xNzA0MTkwNDIxMTJaMHQxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxp
Zm9ybmlhMQ0wCwYDVQQHDARCbGFoMRIwEAYDVQQKDAlTdHVmZiBJbmMxLTArBgNV
BAMMJGx1Y2ktdG9rZW4tc2VydmVyLXRlc3QtMS5mYWtlLmRvbWFpbjCCASIwDQYJ
KoZIhvcNAQEBBQADggEPADCCAQoCggEBAL3Or+NiOG2namAPiBfXTAfN/ENpdiH5
UqQPMUguTu16w8GtUWAqj6vjDiCGjFIKgTCc9DpjckJG7US6t0LuEOOFCG3asst+
K4+5HMP/bfyvcftp3j4cdLX3c/uS4+lIkUSOxUQovXLPpoEkKrgg2gVvCeZd5QmR
J3vH6Rzztx8ZfnZFF9GPE9mQrJ9fYZ1e2e8G3zdXVLkYoVw/lMDdmsaKmdBCdhjU
i2AM9xx5CnOhCzNS8W4jg/SccPMG/uVZ6SLkd5eNuhXLsfREBnsC2RU8Y0Uuw0Ty
//S9MRPskSfyNOFMib+FrPfH2Kf7wDPdtlrdtT/T8ur4J/l48s9gKMkCAwEAAaOB
xTCBwjAJBgNVHRMEAjAAMBEGCWCGSAGG+EIBAQQEAwIFoDAzBglghkgBhvhCAQ0E
JhYkT3BlblNTTCBHZW5lcmF0ZWQgQ2xpZW50IENlcnRpZmljYXRlMB0GA1UdDgQW
BBQf/Xtn7MQpybujv9/54DxdiNDKFzAfBgNVHSMEGDAWgBRhO7licgHsGIwDmWmP
zL+oymoPHjAOBgNVHQ8BAf8EBAMCBeAwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsG
AQUFBwMEMA0GCSqGSIb3DQEBCwUAA4IBAQA9YXLIMwJbfQMMpTxPZLQoiqbG0fPB
xuSBGNYd/US6NIWLOg/v5tfN2GB+RAuB1Rz12eS+TmN7+A/lfNx0dFYwcfeOz05g
jQMwgUDmlnicMqENd0pswccS/mci215addFq6Wknti+To+TST0Ci5zmIt2fbBjmI
VRAWsPfLInwtW94S54UF38n2gp3iXizQLG2urSqotPsWIiyO+f2M3Q2ki3fDzimj
EyA+GFsGD6l0nQUySNyk2xE4S5CHOyLG0qWOsaJsEkTMnN+lrUh1bLUcI3bvVpVP
uwi+mmV6pbwEPKYNHpxHXSbEFnWwnZm1OtM28sP9O0D94XzRq2OfWiiD
-----END CERTIFICATE-----`
