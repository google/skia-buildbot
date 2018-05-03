package main

/*
	Obtain an OAuth2 token from a service account key. Expects to run as root in a cron job.
*/

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	compute "google.golang.org/api/compute/v1"
)

var (
	// Flags.
	serviceAccounts = common.NewMultiStringFlag("service_account", nil, "Mapping of \"/path/to/service_account.key:/path/to/token.file\"")
)

// checkFilePerms returns an error if the given file is not owned and readable
// only by root. Requires that we're running on Linux.
func checkFilePerms(fp string) error {
	info, err := os.Stat(fp)
	if err != nil {
		return fmt.Errorf("Failed to read file: %s", err)
	}
	fi, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("Got wrong type for FileInfo.Sys()!")
	}
	if fi.Uid != 0 {
		return fmt.Errorf("File is not owned by root (UID = %d).", fi.Uid)
	}
	if fi.Gid != 0 {
		return fmt.Errorf("File is not owned by root group (GID = %d).", fi.Gid)
	}
	if info.Mode() != 0600 {
		return fmt.Errorf("File mode must be 0600 (is %d).", info.Mode())
	}
	return nil
}

func processServiceAccount(sa string) error {
	split := strings.Split(sa, ":")
	if len(split) != 2 {
		return fmt.Errorf("Invalid argument for --service_account: %s", sa)
	}
	serviceAccountFile := split[0]
	dest := split[1]

	// Verify that the key file is owned and readable only by root.
	if err := checkFilePerms(serviceAccountFile); err != nil {
		sklog.Fatal(err)
	}

	if dest == "" {
		sklog.Fatalf("--dest is required.")
	}

	src, err := auth.NewJWTServiceAccountTokenSource("#bogus", serviceAccountFile, compute.CloudPlatformScope, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		sklog.Fatal(err)
	}
	tok, err := src.Token()
	if err != nil {
		sklog.Fatal(err)
	}

	b, err := json.Marshal(tok)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := ioutil.WriteFile(dest, b, 0644); err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Wrote new auth token: %s", tok.AccessToken[len(tok.AccessToken)-8:])
	return nil
}

func main() {
	common.Init()

	sklog.Infof("Obtaining new auth token.")

	if *serviceAccounts == nil {
		sklog.Fatalf("At least one --service_account is required.")
	}
	for _, sa := range *serviceAccounts {
		if err := processServiceAccount(sa); err != nil {
			sklog.Fatal(err)
		}
	}
}
