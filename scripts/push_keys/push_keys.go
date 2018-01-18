package main

/*
	Regenerate service account keys and push them to jumphosts.
*/

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	admin "cloud.google.com/go/iam/admin/apiv1"
	"github.com/google/uuid"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	adminpb "google.golang.org/genproto/googleapis/iam/admin/v1"
)

const (
	SSH_HOSTNAME = "jumphost-linux-01" // TODO
	SSH_PORT     = "22"
	SSH_USER     = "chrome-bot"
)

var (
	borenetTesting = &serviceAccount{
		project: "skia-swarming-bots",
		email:   "borenet-testing@skia-swarming-bots.iam.gserviceaccount.com",
	}

	// List of all service accounts whose keys are managed by this script.
	serviceAccounts = []*serviceAccount{
		borenetTesting,
	}

	// Determines which keys go on which machines:
	// map[jumphost_name][service_account_nickname]*serviceAccount
	mapping = map[string]map[string]*serviceAccount{
		"linux-01.skolo": map[string]*serviceAccount{
			"testing": borenetTesting,
		},
	}

	// Flags.
	outdir = flag.String("outdir", ".", "Directory for writing output. Key files will be organized by host.")
)

type serviceAccount struct {
	project string
	email   string
	key     []byte
}

// writeEncryptedFile uses GnuPG to write the file, encrypted using the given
// password.
func writeEncryptedFile(dest, pw string, contents string) error {
	// gpg won't decrypt the file if it doesn't have the .gpg suffix.
	if !strings.HasSuffix(dest, ".gpg") {
		dest += ".gpg"
	}
	cmd := exec.Command("gpg", "-c", "--passphrase-fd", "0", "-o", dest)
	// TODO(borenet): This isn't working?
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = ioutil.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	// Enter the password first.
	if _, err := io.WriteString(stdin, pw+"\n"); err != nil {
		return err
	}
	// Now write the contents.
	if _, err := io.WriteString(stdin, contents+"\n"); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func main() {
	// Setup.
	common.Init()
	ctx := context.Background()
	c, err := admin.NewIamClient(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create new keys for all service accounts. Track the previous keys so
	// that we can delete them later.
	deleteKeys := []string{}
	for _, acc := range serviceAccounts {
		serviceAccountPath := admin.IamServiceAccountPath(acc.project, acc.email)
		resp, err := c.ListServiceAccountKeys(ctx, &adminpb.ListServiceAccountKeysRequest{
			Name: serviceAccountPath,
		})
		if err != nil {
			sklog.Fatal(err)
		}
		for _, key := range resp.Keys {
			validAfter := time.Unix(key.ValidAfterTime.Seconds, 0)
			validBefore := time.Unix(key.ValidBeforeTime.Seconds, 0)
			duration := validBefore.Sub(validAfter)
			if duration > 50*time.Hour {
				deleteKeys = append(deleteKeys, key.Name)
			}
		}

		newKey, err := c.CreateServiceAccountKey(ctx, &adminpb.CreateServiceAccountKeyRequest{
			Name: serviceAccountPath,
		})
		if err != nil {
			sklog.Fatal(err)
		}
		acc.key = newKey.PrivateKeyData
	}

	// Write the new keys to files, organized by jumphost.
	// We write encrypted files, protected with a generated password.
	pw := uuid.New().String()
	fmt.Println("Decryption passphrase:", pw)
	for jumphost, subMap := range mapping {
		destDir := path.Join(*outdir, jumphost)
		if err := os.Mkdir(destDir, os.ModePerm); err != nil {
			sklog.Fatal(err)
		}
		for nickname, acc := range subMap {
			dest := path.Join(destDir, fmt.Sprintf("service_account_%s.json", nickname))
			if err := writeEncryptedFile(dest, pw, string(acc.key)); err != nil {
				sklog.Fatal(err)
			}
		}
	}

	// Delete the old keys.
	for _, key := range deleteKeys {
		sklog.Infof("Deleting %s", key)
		if err := c.DeleteServiceAccountKey(ctx, &adminpb.DeleteServiceAccountKeyRequest{
			Name: key,
		}); err != nil {
			sklog.Fatal(err)
		}
	}
}
