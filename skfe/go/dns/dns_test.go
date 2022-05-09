// Unit tests for our Cloud DNS configuration. This tests the configuration of
// the running configuration of our zone. See the skia.org.zone file for more
// details.
package dns

import (
	"fmt"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

// Find the name of the nameservers to use for Cloud DNS by running:
//
//     gcloud dns managed-zones describe skia-org
//
var allNameServers = []string{
	// These names come from running `gcloud dns managed-zones describe skia-org`.
	"ns-cloud-c1.googledomains.com:53",
	"ns-cloud-c2.googledomains.com:53",
	"ns-cloud-c3.googledomains.com:53",
	"ns-cloud-c4.googledomains.com:53",

	// These are the current nameservers for skia.org, and can be removed after
	// the migration to Cloud DNS is complete.
	"ns1.googledomains.com:53",
	"ns3.googledomains.com:53",
	"ns2.googledomains.com:53",
	"ns4.googledomains.com:53",
}

// testZoneEntry tests a single DNS Zone entry.
//
// qType is the record type, and constants for this variable are defined in the
// dns module.
func testZoneEntry(t *testing.T, qType uint16, domainName, expectedValue string) {
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
		for _, nameserver := range allNameServers {
			response, _, err := dnsClient.Exchange(dnsMsg, nameserver)
			require.NoError(t, err, "nameserver: %q", nameserver)
			require.Contains(t, response.Answer[0].String(), expectedValue, "nameserver: %q", nameserver)
		}
	})

}

func TestDNSConfiguration(t *testing.T) {
	unittest.MediumTest(t)

	// Keep these tests in the same order as the records appear in
	// skia.org.zone, to make it easier to confirm that all cases are being
	// tested.
	testZoneEntry(t, dns.TypeMX, "skia.org.", "smtp.google.com.")

	testZoneEntry(t, dns.TypeCAA, "skia.org.", "pki.goog")

	testZoneEntry(t, dns.TypeCNAME, "_validate_domain.skia.org.", "_validate_domain.pki.goog.")

	testZoneEntry(t, dns.TypeA, "skia.org.", "35.201.76.220")

	testZoneEntry(t, dns.TypeTXT, "skia.org.", "v=spf1 include:_spf.google.com ~all")

	testZoneEntry(t, dns.TypeTXT, "_dmarc.skia.org.", "v=DMARC1; p=reject; rua=mailto:mailauth-reports@google.com")

	testZoneEntry(t, dns.TypeCNAME, "em258.skia.org.", "u26644806.wl057.sendgrid.net.")
	testZoneEntry(t, dns.TypeCNAME, "s1._domainkey.skia.org.", "s1.domainkey.u26644806.wl057.sendgrid.net.")
	testZoneEntry(t, dns.TypeCNAME, "s2._domainkey.skia.org.", "s2.domainkey.u26644806.wl057.sendgrid.net.")
	testZoneEntry(t, dns.TypeCNAME, "url9405.skia.org.", "sendgrid.net.")
	testZoneEntry(t, dns.TypeCNAME, "26644806.skia.org.", "sendgrid.net.")

	testZoneEntry(t, dns.TypeCNAME, "some-random-sub-domain.skia.org.", "skia.org.")
}
