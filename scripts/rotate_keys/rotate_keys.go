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
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	adminpb "google.golang.org/genproto/googleapis/iam/admin/v1"
)

var (
	// Default service account used for bots which connect to
	// chrome-swarming.appspot.com.
	chromeSwarming = &serviceAccount{
		project:  "google.com:skia-buildbots",
		email:    "chrome-swarming-bots@skia-buildbots.google.com.iam.gserviceaccount.com",
		nickname: "swarming",
	}

	// Default service account used for bots which connect to
	// chromium-swarm.appspot.com.
	chromiumSwarm = &serviceAccount{
		project:  "skia-swarming-bots",
		email:    "chromium-swarm-bots@skia-swarming-bots.iam.gserviceaccount.com",
		nickname: "swarming",
	}

	// Service account used by the jumphost itself.
	jumphost = &serviceAccount{
		project:  "google.com:skia-buildbots",
		email:    "jumphost@skia-buildbots.google.com.iam.gserviceaccount.com",
		nickname: "jumphost",
	}

	// Service account used by the RPi masters.
	rpiMaster = &serviceAccount{
		project:  "google.com:skia-buildbots",
		email:    "rpi-master@skia-buildbots.google.com.iam.gserviceaccount.com",
		nickname: "rpi-master",
	}

	// Determines which keys go on which machines:
	// map[jumphost_name][]*serviceAccount
	jumphostServiceAccountMapping = map[string][]*serviceAccount{
		"internal-01.skolo": []*serviceAccount{
			chromeSwarming,
			jumphost,
			rpiMaster,
		},
		"linux-01.skolo": []*serviceAccount{
			chromiumSwarm,
			jumphost,
		},
		"rpi-01.skolo": []*serviceAccount{
			chromiumSwarm,
			jumphost,
			rpiMaster,
		},
		"win-02.skolo": []*serviceAccount{
			chromiumSwarm,
			jumphost,
		},
		"win-03.skolo": []*serviceAccount{
			chromiumSwarm,
			jumphost,
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

	// Create new keys for all service accounts. Track the previous keys so
	// that we can delete them later.
	fmt.Println("Generating new keys.")
	clients := map[string]*admin.IamClient{}
	serviceAccounts := map[*serviceAccount]bool{}
	for _, accounts := range jumphostServiceAccountMapping {
		for _, acc := range accounts {
			serviceAccounts[acc] = true
			if _, ok := clients[acc.project]; !ok {
				tokenSource := auth.NewGCloudTokenSource(acc.project)
				creds := &google.Credentials{
					ProjectID:   acc.project,
					TokenSource: tokenSource,
				}
				c, err := admin.NewIamClient(ctx, option.WithCredentials(creds))
				if err != nil {
					sklog.Fatalf("Failed to create IAM client: %s", err)
				}
				clients[acc.project] = c
			}
		}
	}
	deleteKeys := map[string][]string{}
	for acc, _ := range serviceAccounts {
		serviceAccountPath := admin.IamServiceAccountPath(acc.project, acc.email)
		c := clients[acc.project]
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
			if duration > 21*24*time.Hour {
				// GCE seems to auto-generate keys with short
				// expirations. These do not show up on the UI.
				// Don't mess with them and only delete the
				// longer-lived keys which we've created.
				deleteKeys[acc.project] = append(deleteKeys[acc.project], key.Name)
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
