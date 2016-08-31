// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/luci/luci-go/appengine/gaetesting"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/tokenserver/appengine/model/shards"
	"github.com/luci/luci-go/tokenserver/appengine/utils"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCRL(t *testing.T) {
	Convey("CRL storage works", t, func() {
		caName := "CA"
		shardCount := 4
		cachingTime := 10 * time.Second

		ctx := gaetesting.TestingContext()
		ctx, clk := testclock.UseTime(ctx, testclock.TestTimeUTC)

		// Prepare a set of CRLs (with holes, to be more close to life)
		crl := &pkix.CertificateList{}
		for i := 1; i < 100; i++ {
			crl.TBSCertList.RevokedCertificates = append(crl.TBSCertList.RevokedCertificates, pkix.RevokedCertificate{
				SerialNumber: big.NewInt(int64(i * 3)),
			})
		}

		// Upload it.
		So(UpdateCRLSet(ctx, caName, shardCount, crl), ShouldBeNil)

		// Use it.
		checker := NewCRLChecker(caName, shardCount, cachingTime)
		for i := 1; i < 300; i++ {
			revoked, err := checker.IsRevokedSN(ctx, big.NewInt(int64(i)))
			So(err, ShouldBeNil)
			So(revoked, ShouldEqual, (i%3) == 0)
		}

		// Cert #1 is revoked now too. It will invalidate one cache shard.
		crl.TBSCertList.RevokedCertificates = append(crl.TBSCertList.RevokedCertificates, pkix.RevokedCertificate{
			SerialNumber: big.NewInt(1),
		})

		// Upload it.
		So(UpdateCRLSet(ctx, caName, shardCount, crl), ShouldBeNil)

		// Old cache is still used.
		revoked, err := checker.IsRevokedSN(ctx, big.NewInt(1))
		So(err, ShouldBeNil)
		So(revoked, ShouldBeFalse)

		// Roll time to invalidate the cache.
		clk.Add(cachingTime * 2)

		// New shard version is fetched.
		revoked, err = checker.IsRevokedSN(ctx, big.NewInt(1))
		So(err, ShouldBeNil)
		So(revoked, ShouldBeTrue)

		// Hit a code path for refetching of an unchanged shard. Pick a SN that
		// doesn't belong to shard where '1' is.
		shardIdx := func(sn int64) int {
			blob, err := utils.SerializeSN(big.NewInt(sn))
			So(err, ShouldBeNil)
			return shards.ShardIndex(blob, shardCount)
		}
		forbiddenIdx := shardIdx(1)
		sn := int64(2)
		for shardIdx(sn) == forbiddenIdx {
			sn++
		}

		// Hit this shard.
		revoked, err = checker.IsRevokedSN(ctx, big.NewInt(sn))
		So(err, ShouldBeNil)
		So(revoked, ShouldEqual, (sn%3) == 0)
	})
}
