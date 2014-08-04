package auth

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/bigquery/v2"
	"github.com/oxtoacart/webbrowser"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

const (
	// TIMEOUT is the http timeout when making BigQuery requests.
	TIMEOUT = time.Duration(time.Minute)
)

var (
	oauthConfig = &oauth.Config{
		ClientId:     "470362608618-nlbqngfl87f4b3mhqqe9ojgaoe11vrld.apps.googleusercontent.com",
		ClientSecret: "J4YCkfMXFJISGyuBuVEiH60T",
		Scope:        bigquery.BigqueryScope,
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		TokenCache:   oauth.CacheFile("bqtoken.data"),
	}
)

// dialTimeout is a dialer that sets a timeout.
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, TIMEOUT)
}

// runFlow runs through a 3LO OAuth 2.0 flow to get credentials for BigQuery.
func RunFlow() (*http.Client, error) {
	transport := &oauth.Transport{
		Config: oauthConfig,
		Transport: &http.Transport{
			Dial: util.DialTimeout,
		},
	}
	if _, err := oauthConfig.TokenCache.Token(); err != nil {
		url := oauthConfig.AuthCodeURL("")
		fmt.Printf(`Your browser has been opened to visit:

  %s

Enter the verification code:`, url)
		webbrowser.Open(url)
		var code string
		fmt.Scan(&code)
		if _, err := transport.Exchange(code); err != nil {
			return nil, err
		}
	}

	return transport.Client(), nil
}
