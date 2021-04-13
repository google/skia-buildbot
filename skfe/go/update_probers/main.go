// update_probers is a command line application that updates the
// "envoy-redirects" section of probersk.json5 with all the redirects in
// envoy-starter.json.
//
package main

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	probersFilename = "probersk.json5"

	envoyFilename = "envoy-starter.json"
)

var (
	proberRedirectURLsPath = []string{"envoy-redirects", "urls"}

	// Domains that won't return a 301 since all the redirects are just for
	// paths below "/"".
	domainsWithJustPrefixRewrites = []string{"skia.org"}
)

func main() {
	// Load the existing probers file.
	probers, err := gabs.ParseJSONFile(probersFilename)
	if err != nil {
		sklog.Fatal(err)
	}

	// Zero our the urls array.
	err = probers.Delete(proberRedirectURLsPath...)
	if err != nil {
		sklog.Fatal(err)
	}
	_, err = probers.Array(proberRedirectURLsPath...)
	if err != nil {
		sklog.Fatal(err)
	}

	// Load the envoy config file.
	redirects, err := gabs.ParseJSONFile(envoyFilename)
	if err != nil {
		sklog.Fatal(err)
	}

	// Find the domain names of all redirects in the envoy file.
	hostsPath := []string{"static_resources", "listeners", "0", "filter_chains", "filters", "0", "typed_config", "route_config", "virtual_hosts"}
	var domains []string
	for _, cluster := range redirects.Search(hostsPath...).Children() {
		domain := cluster.Search("domains").Data().(string)
		// Is there a routes redirect?
		redirect := false
		for _, route := range cluster.Search("routes").Children() {
			if route.Exists("redirect") {
				redirect = true
				break
			}
		}
		// Should we skip this domain?
		if util.In(domain, domainsWithJustPrefixRewrites) {
			redirect = false
		}

		if redirect && domain != "*" {
			domains = append(domains, fmt.Sprintf("https://%s", strings.TrimSpace(domain)))
		}
	}

	// Add the domains to the probers file.
	sort.Strings(domains)
	for _, domain := range domains {

		if err := probers.ArrayAppend(domain, proberRedirectURLsPath...); err != nil {
			sklog.Fatal(err)
		}
	}

	// Rewrite the probers file.
	if err := ioutil.WriteFile(probersFilename, []byte(probers.StringIndent("", "  ")), 0644); err != nil {
		sklog.Fatal(err)
	}
}
