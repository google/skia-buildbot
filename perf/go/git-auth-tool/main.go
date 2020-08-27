// git-auth-tool is a simple command line application that sets up git
// authentication using the default token source.
package main

import (
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/sklog"
)

func main() {
	ts, err := auth.NewDefaultTokenSource(false, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	_, err = gitauth.New(ts, "/tmp/git-cookie", true, "")
	if err != nil {
		sklog.Fatal(err)
	}
}
