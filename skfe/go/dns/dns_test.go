// Unit tests for our Cloud DNS configuration. This tests the configuration of
// the running configuration of our zone. See the skia.org.zone file for more
// details.
package dns

import (
	"fmt"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

// Find the name of the nameservers to use for Cloud DNS by running:
//
//	gcloud dns managed-zones describe skia-org
var allSkiaOrgNameServers = []string{
	// These names come from running `gcloud dns managed-zones describe skia-org`.
	"ns-cloud-c1.googledomains.com:53",
	"ns-cloud-c2.googledomains.com:53",
	"ns-cloud-c3.googledomains.com:53",
	"ns-cloud-c4.googledomains.com:53",
}

var allLuciAppNameServers = []string{
	// These names come from running `gcloud dns managed-zones describe luci-app`.
	"ns-cloud-b1.googledomains.com:53",
	"ns-cloud-b2.googledomains.com:53",
	"ns-cloud-b3.googledomains.com:53",
	"ns-cloud-b4.googledomains.com:53",
}

// testSkiaOrgZoneEntry tests a single DNS Zone entry.
//
// qType is the record type, and constants for this variable are defined in the
// dns module.
func testSkiaOrgZoneEntry(t *testing.T, qType uint16, domainName, expectedValue string) {
	testZoneEntry(t, qType, domainName, expectedValue, allSkiaOrgNameServers)
}

// testLuciAppZoneEntry tests a single DNS Zone entry.
//
// qType is the record type, and constants for this variable are defined in the
// dns module.
func testLuciAppZoneEntry(t *testing.T, qType uint16, domainName, expectedValue string) {
	testZoneEntry(t, qType, domainName, expectedValue, allLuciAppNameServers)
}

// testSkiaOrgZoneEntry tests a single DNS Zone entry.
//
// qType is the record type, and constants for this variable are defined in the
// dns module.
func testZoneEntry(t *testing.T, qType uint16, domainName, expectedValue string, nameservers []string) {
	dnsMsg := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:               dns.Id(),
			RecursionDesired: false,
		},
		Question: []dns.Question{
			{
				Name:   domainName,
				Qtype:  qType,
				Qclass: dns.ClassINET,
			},
		},
	}
	dnsClient := dns.Client{}

	t.Run(fmt.Sprintf("%s_%d", domainName, qType), func(t *testing.T) {
		for _, nameserver := range nameservers {
			response, _, err := dnsClient.Exchange(dnsMsg, nameserver)
			require.NoError(t, err, "nameserver: %q", nameserver)
			require.Contains(t, response.Answer[0].String(), expectedValue, "nameserver: %q", nameserver)
		}
	})

}

func TestSkiaOrgDNSConfiguration(t *testing.T) {

	// Keep these tests in the same order as the records appear in
	// skia.org.zone, to make it easier to confirm that all cases are being
	// tested.
	testSkiaOrgZoneEntry(t, dns.TypeMX, "skia.org.", "smtp.google.com.")

	testSkiaOrgZoneEntry(t, dns.TypeCAA, "skia.org.", "pki.goog")

	testSkiaOrgZoneEntry(t, dns.TypeCNAME, "_validate_domain.skia.org.", "_validate_domain.pki.goog.")

	testSkiaOrgZoneEntry(t, dns.TypeA, "autoroll.skia.org.", "34.110.212.89")
	testSkiaOrgZoneEntry(t, dns.TypeA, "status.skia.org.", "34.110.212.89")
	testSkiaOrgZoneEntry(t, dns.TypeA, "cabe.skia.org.", "34.110.212.89")
	testSkiaOrgZoneEntry(t, dns.TypeA, "perf-infra-public-cdb.skia.org.", "34.110.212.89")
	testSkiaOrgZoneEntry(t, dns.TypeA, "envoy-admin-panel-public.skia.org.", "34.110.212.89")

	testSkiaOrgZoneEntry(t, dns.TypeA, "skia.org.", "35.201.76.220")

	testSkiaOrgZoneEntry(t, dns.TypeTXT, "skia.org.", "v=spf1 include:_spf.google.com ~all")

	testSkiaOrgZoneEntry(t, dns.TypeTXT, "_dmarc.skia.org.", "v=DMARC1; p=reject; rua=mailto:mailauth-reports@google.com")

	testSkiaOrgZoneEntry(t, dns.TypeCNAME, "em258.skia.org.", "u26644806.wl057.sendgrid.net.")
	testSkiaOrgZoneEntry(t, dns.TypeCNAME, "s1._domainkey.skia.org.", "s1.domainkey.u26644806.wl057.sendgrid.net.")
	testSkiaOrgZoneEntry(t, dns.TypeCNAME, "s2._domainkey.skia.org.", "s2.domainkey.u26644806.wl057.sendgrid.net.")
	testSkiaOrgZoneEntry(t, dns.TypeCNAME, "url9405.skia.org.", "sendgrid.net.")
	testSkiaOrgZoneEntry(t, dns.TypeCNAME, "26644806.skia.org.", "sendgrid.net.")

	testSkiaOrgZoneEntry(t, dns.TypeCNAME, "some-random-sub-domain.skia.org.", "skia.org.")
}

func TestLuciAppDNSConfiguration(t *testing.T) {

	// Keep these tests in the same order as the records appear in
	// luci.app.zone, to make it easier to confirm that all cases are being
	// tested.

	testLuciAppZoneEntry(t, dns.TypeCAA, "luci.app.", "pki.goog")
	testLuciAppZoneEntry(t, dns.TypeCNAME, "_validate_domain.luci.app.", "_validate_domain.pki.goog.")
	testLuciAppZoneEntry(t, dns.TypeA, "luci.app.", "34.110.212.89")
	testLuciAppZoneEntry(t, dns.TypeCNAME, "some-random-sub-domain.luci.app.", "luci.app.")
}
