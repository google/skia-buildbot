package main

import (
	"encoding/json"
	"io"
	"runtime"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

/*
	Obtain an OAuth token from metadata and write it to the expected location.

	Run this program every 4 minutes to ensure that the token is always valid.
*/

const (
	TOKEN_DEST     = "/var/lib/swarming/oauth_bot_token.json"
	TOKEN_DEST_WIN = "C:\\swarming\\oauth_bot_token.json"
)

func main() {
	common.Init()
	skiaversion.MustLogVersion()

	// Obtain the token.
	tok, err := metadata.GetToken()
	if err != nil {
		sklog.Fatal(err)
	}

	// Swarming expects tokens in a slightly different format.
	token := struct {
		Token     string `json:"token"`
		TokenType string `json:"token_type"`
		Expiry    int64  `json:"expiry"`
	}{
		Token:     tok.AccessToken,
		TokenType: tok.TokenType,
		Expiry:    tok.Expiry.Unix(),
	}

	// Write the token to the expected location.
	dest := TOKEN_DEST
	if runtime.GOOS == "windows" {
		dest = TOKEN_DEST_WIN
	}
	if err := util.WithWriteFile(dest, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(token)
	}); err != nil {
		sklog.Fatal(err)
	}
}
