package main

/*
	Regenerate service account keys and copy them to the jumphosts.
*/

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	admin "cloud.google.com/go/iam/admin/apiv1"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	adminpb "google.golang.org/genproto/googleapis/iam/admin/v1"
)

var (
	// Default service account used for bots which connect to
	// chromium-swarm.appspot.com.
	chromiumSwarm = &serviceAccount{
		project:  "skia-swarming-bots",
		email:    "chromium-swarm-bots@skia-swarming-bots.iam.gserviceaccount.com",
		nickname: "swarming",
	}

	// Determines which keys go on which machines:
	// map[jumphost_name][]*serviceAccount
	jumphostServiceAccountMapping = map[string][]*serviceAccount{
		"linux-01.skolo": []*serviceAccount{
			chromiumSwarm,
		},
		"rpi-01.skolo": []*serviceAccount{
			chromiumSwarm,
		},
		"win-01.skolo": []*serviceAccount{
			chromiumSwarm,
		},
		"win-02.skolo": []*serviceAccount{
			chromiumSwarm,
		},
	}
)

// serviceAccount is a struct representing a service account.
type serviceAccount struct {
	project  string
	email    string
	nickname string
	key      []byte
}

// copyKey copies the given service account key to the given jumphost.
func copyKey(hostname, filepath string, key []byte) error {
	fmt.Println("Copying", filepath, "to", hostname)
	sshCmd := fmt.Sprintf("touch %s && chmod 600 %s && chown root:root %s && cat > %s", filepath, filepath, filepath, filepath)
	cmd := exec.Command("ssh", fmt.Sprintf("root@%s", hostname), sshCmd)
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
	c, err := admin.NewIamClient(ctx)
	if err != nil {
		sklog.Fatalf("Failed to create IAM client: %s", err)
	}

	// Create new keys for all service accounts. Track the previous keys so
	// that we can delete them later.
	fmt.Println("Generating new keys.")
	serviceAccounts := map[*serviceAccount]bool{}
	for _, accounts := range jumphostServiceAccountMapping {
		for _, acc := range accounts {
			serviceAccounts[acc] = true
		}
	}
	deleteKeys := []string{}
	for acc, _ := range serviceAccounts {
		serviceAccountPath := admin.IamServiceAccountPath(acc.project, acc.email)
		resp, err := c.ListServiceAccountKeys(ctx, &adminpb.ListServiceAccountKeysRequest{
			Name: serviceAccountPath,
		})
		if err != nil {
			sklog.Fatalf("Failed to list keys for %s: %s", acc.email, err)
		}
		for _, key := range resp.Keys {
			validAfter := time.Unix(key.ValidAfterTime.Seconds, 0)
			validBefore := time.Unix(key.ValidBeforeTime.Seconds, 0)
			duration := validBefore.Sub(validAfter)
			if duration > 50*time.Hour {
				// GCE seems to auto-generate keys with 48-hour
				// expirations. These do not show up on the UI.
				// Don't mess with them and only delete the
				// longer-lived keys which we've created.
				deleteKeys = append(deleteKeys, key.Name)
			}
		}

		newKey, err := c.CreateServiceAccountKey(ctx, &adminpb.CreateServiceAccountKeyRequest{
			Name: serviceAccountPath,
		})
		if err != nil {
			sklog.Fatalf("Failed to create a new key for %s: %s", acc.email, err)
		}
		acc.key = newKey.PrivateKeyData
	}

	// Copy the key files to the jumphosts.
	for jumphost, accounts := range jumphostServiceAccountMapping {
		for _, acc := range accounts {
			if err := copyKey(jumphost, fmt.Sprintf("/etc/service_account_%s.json", acc.nickname), acc.key); err != nil {
				sklog.Fatalf("Failed to copy key for %s to %s: %s", acc.email, jumphost, err)
			}
		}
	}

	// Delete the old keys.
	for _, key := range deleteKeys {
		fmt.Println("Deleting", key)
		if err := c.DeleteServiceAccountKey(ctx, &adminpb.DeleteServiceAccountKeyRequest{
			Name: key,
		}); err != nil {
			sklog.Fatalf("Failed to delete service account key %q: %s", key, err)
		}
	}
}
