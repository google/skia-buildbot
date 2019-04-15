package main

/*
	Regenerate service account keys and copy them to the jumphosts.
*/

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	admin "cloud.google.com/go/iam/admin/apiv1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/service_accounts"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	adminpb "google.golang.org/genproto/googleapis/iam/admin/v1"
)

var (
	nicknames = common.NewMultiStringFlag("account", nil, "Nicknames of service accounts whose keys should be rotated. If not specified, rotate all keys.")
)

// copyKey copies the given service account key to the given jumphost.
func copyKey(hostname, filepath string, key []byte) error {
	fmt.Println("Copying", filepath, "to", hostname)
	sshCmd := fmt.Sprintf("touch %s && chmod 600 %s && chown root:root %s && cat > %s", filepath, filepath, filepath, filepath)
	sshName := service_accounts.JumphostSSHMapping[hostname]
	cmd := exec.Command("ssh", fmt.Sprintf("root@%s", sshName), sshCmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := io.WriteString(stdin, string(key)+"\n"); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}

func main() {
	// Setup.
	common.Init()
	ctx := context.Background()

	// Validate the requested service account nicknames.
	validAccts := map[string]bool{}
	for _, accounts := range service_accounts.JumphostServiceAccountMapping {
		for _, acct := range accounts {
			validAccts[acct.Nickname] = true
		}
	}
	requestedAccts := map[string]bool{}
	if len(*nicknames) > 0 {
		for _, nickname := range *nicknames {
			if _, ok := validAccts[nickname]; !ok {
				sklog.Fatalf("Unknown service account %q", nickname)
			}
			requestedAccts[nickname] = true
		}
	} else {
		// If no account was specified, ask the user if they're sure and
		// then rotate keys for all accounts.
		fmt.Printf("Are you sure you want to rotate keys for all " +
			"service accounts? Individual accounts can be " +
			"specified using --account. [y/n]: ")
		r := bufio.NewReader(os.Stdin)
		resp, err := r.ReadString('\n')
		if err != nil {
			sklog.Fatal(err)
		}
		if strings.ToLower(strings.TrimSpace(resp)) != "y" {
			return
		}
		requestedAccts = validAccts
	}
	fmt.Println("Rotating service account keys for:")
	for acct := range requestedAccts {
		fmt.Println("  " + acct)
	}

	// Create new keys for all service accounts. Track the previous keys so
	// that we can delete them later.
	fmt.Println("Generating new keys.")
	clients := map[string]*admin.IamClient{}
	serviceAccounts := map[*service_accounts.ServiceAccount]bool{}
	for _, accounts := range service_accounts.JumphostServiceAccountMapping {
		for _, acc := range accounts {
			if _, ok := requestedAccts[acc.Nickname]; !ok {
				continue
			}

			serviceAccounts[acc] = true
			if _, ok := clients[acc.Project]; !ok {
				tokenSource := auth.NewGCloudTokenSource(acc.Project)
				creds := &google.Credentials{
					ProjectID:   acc.Project,
					TokenSource: tokenSource,
				}
				c, err := admin.NewIamClient(ctx, option.WithCredentials(creds))
				if err != nil {
					sklog.Fatalf("Failed to create IAM client: %s", err)
				}
				clients[acc.Project] = c
			}
		}
	}
	deleteKeys := map[string][]string{}
	newKeys := map[*service_accounts.ServiceAccount][]byte{}
	for acc := range serviceAccounts {
		fmt.Println("Creating new key for " + acc.Email)
		serviceAccountPath := admin.IamServiceAccountPath(acc.Project, acc.Email)
		c := clients[acc.Project]
		resp, err := c.ListServiceAccountKeys(ctx, &adminpb.ListServiceAccountKeysRequest{
			Name: serviceAccountPath,
		})
		if err != nil {
			sklog.Fatalf("Failed to list keys for %s: %s", acc.Email, err)
		}
		for _, key := range resp.Keys {
			validAfter := time.Unix(key.ValidAfterTime.Seconds, 0)
			validBefore := time.Unix(key.ValidBeforeTime.Seconds, 0)
			duration := validBefore.Sub(validAfter)
			if duration > 21*24*time.Hour {
				// GCE seems to auto-generate keys with short
				// expirations. These do not show up on the UI.
				// Don't mess with them and only delete the
				// longer-lived keys which we've created.
				deleteKeys[acc.Project] = append(deleteKeys[acc.Project], key.Name)
			}
		}

		newKey, err := c.CreateServiceAccountKey(ctx, &adminpb.CreateServiceAccountKeyRequest{
			Name: serviceAccountPath,
		})
		if err != nil {
			sklog.Fatalf("Failed to create a new key for %s: %s", acc.Email, err)
		}
		newKeys[acc] = newKey.PrivateKeyData
	}

	// Copy the key files to the jumphosts.
	for jumphost, accounts := range service_accounts.JumphostServiceAccountMapping {
		for _, acc := range accounts {
			if err := copyKey(jumphost, fmt.Sprintf("/etc/service_account_%s.json", acc.Nickname), newKeys[acc]); err != nil {
				sklog.Fatalf("Failed to copy key for %s to %s: %s", acc.Email, jumphost, err)
			}
		}
	}

	// Delete the old keys.
	for project, keys := range deleteKeys {
		for _, key := range keys {
			fmt.Println("Deleting", key)
			c := clients[project]
			if err := c.DeleteServiceAccountKey(ctx, &adminpb.DeleteServiceAccountKeyRequest{
				Name: key,
			}); err != nil {
				sklog.Fatalf("Failed to delete service account key %q: %s", key, err)
			}
		}
	}
}
