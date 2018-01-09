package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"syscall"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	// Flags.
	keyfile = flag.String("keyfile", "", "Path to the service account key JSON file.")
	dest    = flag.String("dest", "", "Destination path to write the token file.")
)

// checkFilePerms returns an error if the given file is not owned and readable
// only by root. Requires that we're running on Linux.
func checkFilePerms(fp string) error {
	info, err := os.Stat(*keyfile)
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

func main() {
	common.Init()

	if *keyfile == "" {
		sklog.Fatalf("--keyfile is required.")
	}

	// Verify that the key file is owned and readable only by root.
	if err := checkFilePerms(*keyfile); err != nil {
		sklog.Fatal(err)
	}

	if *dest == "" {
		sklog.Fatalf("--dest is required.")
	}

	src, err := auth.NewJWTServiceAccountTokenSource("#bogus", *keyfile, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		sklog.Fatal(err)
	}
	tok, err := src.Token()
	if err != nil {
		sklog.Fatal(err)
	}

	if err := util.WithWriteFile(*dest, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(tok)
	}); err != nil {
		sklog.Fatal(err)
	}
}
