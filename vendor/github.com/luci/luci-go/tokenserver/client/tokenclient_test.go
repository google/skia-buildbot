// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package client

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/common/proto/google"

	"github.com/luci/luci-go/tokenserver/api/minter/v1"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTokenClient(t *testing.T) {
	Convey("works", t, func() {
		ctx := context.Background()
		ctx, _ = testclock.UseTime(ctx, time.Date(2015, time.February, 3, 4, 5, 6, 7, time.UTC))

		expectedResp := &minter.MachineTokenResponse{
			TokenType: &minter.MachineTokenResponse_LuciMachineToken{
				LuciMachineToken: &minter.LuciMachineToken{
					MachineToken: "blah",
				},
			},
		}

		c := Client{
			Client: &fakeRPCClient{
				Out: minter.MintMachineTokenResponse{TokenResponse: expectedResp},
			},
			Signer: &fakeSigner{},
		}

		resp, err := c.MintMachineToken(ctx, &minter.MachineTokenRequest{
			TokenType:    minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			Oauth2Scopes: []string{"scope1", "scope2"},
		})
		So(err, ShouldBeNil)
		So(resp, ShouldResemble, expectedResp)

		rpc := c.Client.(*fakeRPCClient).In
		So(rpc.Signature, ShouldResemble, []byte("fake signature"))

		tokReq := minter.MachineTokenRequest{}
		So(proto.Unmarshal(rpc.SerializedTokenRequest, &tokReq), ShouldBeNil)
		So(tokReq, ShouldResemble, minter.MachineTokenRequest{
			Certificate:        []byte("fake certificate"),
			SignatureAlgorithm: minter.SignatureAlgorithm_SHA256_RSA_ALGO,
			IssuedAt:           google.NewTimestamp(clock.Now(ctx)),
			TokenType:          minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			Oauth2Scopes:       []string{"scope1", "scope2"},
		})
	})

	Convey("handles error", t, func() {
		ctx := context.Background()

		c := Client{
			Client: &fakeRPCClient{
				Out: minter.MintMachineTokenResponse{
					ErrorCode:    1234,
					ErrorMessage: "blah",
				},
			},
			Signer: &fakeSigner{},
		}

		_, err := c.MintMachineToken(ctx, &minter.MachineTokenRequest{
			TokenType:    minter.TokenType_GOOGLE_OAUTH2_ACCESS_TOKEN,
			Oauth2Scopes: []string{"scope1", "scope2"},
		})
		So(err.Error(), ShouldEqual, "token server error 1234 - blah")
	})
}

// fakeRPCClient implements minter.TokenMinterClient.
type fakeRPCClient struct {
	In  minter.MintMachineTokenRequest
	Out minter.MintMachineTokenResponse
}

func (f *fakeRPCClient) MintMachineToken(ctx context.Context, in *minter.MintMachineTokenRequest, opts ...grpc.CallOption) (*minter.MintMachineTokenResponse, error) {
	f.In = *in
	return &f.Out, nil
}

func (f *fakeRPCClient) InspectMachineToken(context.Context, *minter.InspectMachineTokenRequest, ...grpc.CallOption) (*minter.InspectMachineTokenResponse, error) {
	panic("not implemented")
}

// fakeSigner implements Signer.
type fakeSigner struct{}

func (f *fakeSigner) Algo(ctx context.Context) (x509.SignatureAlgorithm, error) {
	return x509.SHA256WithRSA, nil
}

func (f *fakeSigner) Certificate(ctx context.Context) ([]byte, error) {
	return []byte("fake certificate"), nil
}

func (f *fakeSigner) Sign(ctx context.Context, blob []byte) ([]byte, error) {
	return []byte("fake signature"), nil
}
