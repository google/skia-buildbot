// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package machine

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/memlogger"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/authtest"
	"github.com/luci/luci-go/server/auth/signing"
	"github.com/luci/luci-go/server/auth/signing/signingtest"

	"github.com/luci/luci-go/tokenserver/api"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMachineTokenAuthMethod(t *testing.T) {
	Convey("with mock context", t, func() {
		ctx := makeTestContext()
		log := logging.Get(ctx).(*memlogger.MemLogger)
		signer := signingtest.NewSigner(0, nil)
		method := MachineTokenAuthMethod{
			certsFetcher: func(c context.Context, email string) (*signing.PublicCertificates, error) {
				if email == "valid-signer@example.com" {
					return signer.Certificates(c)
				}
				return nil, fmt.Errorf("unknown signer")
			},
		}

		mint := func(tok *tokenserver.MachineTokenBody, sig []byte) string {
			body, _ := proto.Marshal(tok)
			keyID, validSig, _ := signer.SignBytes(ctx, body)
			if sig == nil {
				sig = validSig
			}
			envelope, _ := proto.Marshal(&tokenserver.MachineTokenEnvelope{
				TokenBody: body,
				KeyId:     keyID,
				RsaSha256: sig,
			})
			return base64.RawStdEncoding.EncodeToString(envelope)
		}

		call := func(tok string) (*auth.User, error) {
			r, _ := http.NewRequest("GET", "/something", nil)
			if tok != "" {
				r.Header.Set(MachineTokenHeader, tok)
			}
			return method.Authenticate(ctx, r)
		}

		hasLog := func(msg string) bool {
			return log.HasFunc(func(m *memlogger.LogEntry) bool {
				return strings.Contains(m.Msg, msg)
			})
		}

		Convey("valid token works", func() {
			user, err := call(mint(&tokenserver.MachineTokenBody{
				MachineFqdn: "some-machine.location",
				IssuedBy:    "valid-signer@example.com",
				IssuedAt:    uint64(clock.Now(ctx).Unix()),
				Lifetime:    3600,
			}, nil))
			So(err, ShouldBeNil)
			So(user, ShouldResemble, &auth.User{Identity: "bot:some-machine.location"})
		})

		Convey("not header => not applicable", func() {
			user, err := call("")
			So(user, ShouldBeNil)
			So(err, ShouldBeNil)
		})

		Convey("not base64 envelope", func() {
			_, err := call("not-a-valid-token")
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Failed to deserialize the token"), ShouldBeTrue)
		})

		Convey("broken envelope", func() {
			_, err := call("abcdef")
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Failed to deserialize the token"), ShouldBeTrue)
		})

		Convey("broken body", func() {
			envelope, _ := proto.Marshal(&tokenserver.MachineTokenEnvelope{
				TokenBody: []byte("bad body"),
				KeyId:     "123",
				RsaSha256: []byte("12345"),
			})
			tok := base64.RawStdEncoding.EncodeToString(envelope)
			_, err := call(tok)
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Failed to deserialize the token"), ShouldBeTrue)
		})

		Convey("bad signer ID", func() {
			_, err := call(mint(&tokenserver.MachineTokenBody{
				MachineFqdn: "some-machine.location",
				IssuedBy:    "not-a-email.com",
				IssuedAt:    uint64(clock.Now(ctx).Unix()),
				Lifetime:    3600,
			}, nil))
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Bad issued_by field"), ShouldBeTrue)
		})

		Convey("unknown signer", func() {
			_, err := call(mint(&tokenserver.MachineTokenBody{
				MachineFqdn: "some-machine.location",
				IssuedBy:    "unknown-signer@example.com",
				IssuedAt:    uint64(clock.Now(ctx).Unix()),
				Lifetime:    3600,
			}, nil))
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Unknown token issuer"), ShouldBeTrue)
		})

		Convey("not yet valid", func() {
			_, err := call(mint(&tokenserver.MachineTokenBody{
				MachineFqdn: "some-machine.location",
				IssuedBy:    "valid-signer@example.com",
				IssuedAt:    uint64(clock.Now(ctx).Unix()) + 60,
				Lifetime:    3600,
			}, nil))
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Token has expired or not yet valid"), ShouldBeTrue)
		})

		Convey("expired", func() {
			_, err := call(mint(&tokenserver.MachineTokenBody{
				MachineFqdn: "some-machine.location",
				IssuedBy:    "valid-signer@example.com",
				IssuedAt:    uint64(clock.Now(ctx).Unix()) - 3620,
				Lifetime:    3600,
			}, nil))
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Token has expired or not yet valid"), ShouldBeTrue)
		})

		Convey("bad signature", func() {
			_, err := call(mint(&tokenserver.MachineTokenBody{
				MachineFqdn: "some-machine.location",
				IssuedBy:    "valid-signer@example.com",
				IssuedAt:    uint64(clock.Now(ctx).Unix()),
				Lifetime:    3600,
			}, []byte("bad signature")))
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Bad signature"), ShouldBeTrue)
		})

		Convey("bad machine_fqdn", func() {
			_, err := call(mint(&tokenserver.MachineTokenBody{
				MachineFqdn: "not::valid::machine::id",
				IssuedBy:    "valid-signer@example.com",
				IssuedAt:    uint64(clock.Now(ctx).Unix()),
				Lifetime:    3600,
			}, nil))
			So(err, ShouldEqual, ErrBadToken)
			So(hasLog("Bad machine_fqdn"), ShouldBeTrue)
		})
	})
}

func makeTestContext() context.Context {
	ctx := context.Background()
	ctx, _ = testclock.UseTime(ctx, time.Date(2015, time.February, 3, 4, 5, 6, 7, time.UTC))
	ctx = memlogger.Use(ctx)

	db := authtest.FakeDB{
		"user:valid-signer@example.com": []string{TokenServersGroup},
	}
	ctx = db.Use(ctx)

	return ctx
}
