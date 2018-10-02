// Runs through a three-legged OAuth 2.0 flow to create a client_token.json
// file which contains a refresh token.
//
// Presumes the client_secret.json file is in the current directory.
//
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// flags
var (
	scopes = flag.String("scopes", "", "A comma separated list of scopes.")
)

func main() {
	common.Init()
	ctx := context.Background()

	var cfg email.ClientSecrets
	err := util.WithReadFile("client_secret.json", func(f io.Reader) error {
		return json.NewDecoder(f).Decode(&cfg)
	})
	if err != nil {
		log.Fatal(err)
	}

	scopeSlice := strings.Split(*scopes, ",")
	conf := &oauth2.Config{
		ClientID:     cfg.Installed.ClientID,
		ClientSecret: cfg.Installed.ClientSecret,
		Scopes:       scopeSlice,
		RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		Endpoint:     google.Endpoint,
	}

	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("Visit the URL for the auth dialog: %v\n", url)

	// Use the authorization code that is pushed to the redirect
	// URL. Exchange will do the handshake to retrieve the
	// initial access token. The HTTP Client returned by
	// conf.Client will refresh the token as necessary.
	fmt.Printf("\nCode: ")
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatal(err)
	}
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.Marshal(tok)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile("client_token.json", b, 0664)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Token written.")
}
