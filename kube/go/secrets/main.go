package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/ramdisk"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var (
	// CLI options.
	cliApp = &cli.App{
		Name:        "secrets",
		Description: "Provides tools for working with secrets.",
		Commands: []*cli.Command{
			{
				Name:        "create",
				Description: "Create a new secret in Cloud Secret Manager.",
				Usage:       "create <project> <secret name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 2 {
						return skerr.Fmt("Expected 2 positional arguments.")
					}
					return app.cmdCreate(ctx.Context, args[0], args[1])
				},
			},
			{
				Name:        "describe",
				Description: "Prints information about a secret in Kubernetes.",
				Usage:       "describe <src project> <src secret name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 2 {
						return skerr.Fmt("Expected 2 positional arguments.")
					}
					return app.cmdDescribe(ctx.Context, args[0], args[1])
				},
			},
			{
				Name:        "migrate",
				Description: "Migrate a secret from Kubernetes to Cloud Secret Manager.",
				Usage:       "migrate <src project> <src secret name> <src secret sub-path> <dst project> <dst secret name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 5 {
						return skerr.Fmt("Expected 5 positional arguments.")
					}
					return app.cmdMigrate(ctx.Context, args[0], args[1], args[2], args[3], args[4])
				},
			},
			{
				Name:        "update",
				Description: "Update a secret in Cloud Secret Manager.",
				Usage:       "update <project> <secret name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 2 {
						return skerr.Fmt("Expected 2 positional arguments.")
					}
					return app.cmdUpdate(ctx.Context, args[0], args[1])
				},
			},
			{
				Name:        "grant-access",
				Description: "Grant access to a secret in Cloud Secret Manager.",
				Usage:       "grant-access <project> <secret name>[,<secret name>] <service account name>[,<service account name>]",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 3 {
						return skerr.Fmt("Expected 3 positional arguments.")
					}
					return app.cmdGrantAccess(ctx.Context, args[0], strings.Split(args[1], ","), strings.Split(args[2], ","))
				},
			},
			{
				Name:        "revoke-access",
				Description: "Revoke access to a secret in Cloud Secret Manager.",
				Usage:       "revoke-access <project> <secret name>[,<secret name>] <service account name>[,<service account name>]",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 3 {
						return skerr.Fmt("Expected 3 positional arguments.")
					}
					return app.cmdRevokeAccess(ctx.Context, args[0], strings.Split(args[1], ","), strings.Split(args[2], ","))
				},
			},
		},
		Usage: "secrets <subcommand>",
	}

	app *secretsApp
)

type secretsApp struct {
	// stdin is an abstraction of os.Stdin which is convenient for testing.
	stdin io.Reader
	// stdout is an abstraction of os.Stdout which is convenient for testing.
	stdout io.Writer

	// secretClient can be mocked for testing.
	secretClient secret.Client
}

// cmdCreate creates a new secret in Cloud Secret Manager.
func (a *secretsApp) cmdCreate(ctx context.Context, project, secretName string) error {
	if err := a.secretClient.Create(ctx, project, secretName); err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(a.updateSecret(ctx, project, secretName, ""))
}

// cmdDescribe prints information about a secret in Kubernetes.
func (a *secretsApp) cmdDescribe(ctx context.Context, srcProject, srcSecretName string) error {
	secretMap, err := getK8sSecret(ctx, srcProject, srcSecretName)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println("Secret contains sub-paths:")
	for key := range secretMap {
		fmt.Println("  " + key)
	}
	return nil
}

// cmdMigrate migrates a secret from Kubernetes to Cloud Secret Manager.
func (a *secretsApp) cmdMigrate(ctx context.Context, srcProject, srcSecretName, srcSecretPath, dstProject, dstSecretName string) error {
	secretMap, err := getK8sSecret(ctx, srcProject, srcSecretName)
	if err != nil {
		return skerr.Wrap(err)
	}
	value, ok := secretMap[srcSecretPath]
	if !ok {
		return skerr.Fmt("secret %q has no value at path %q", srcSecretName, srcSecretPath)
	}
	// Store the secret.
	// Note: because this tool is designed for first-time migration of secrets,
	// we're unconditionally creating the secret first.  Creation of a secret
	// which already exists should fail, which is what we want for this use
	// case.
	if err := a.secretClient.Create(ctx, dstProject, dstSecretName); err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(a.putSecret(ctx, dstProject, dstSecretName, value))
}

// cmdUpdate updates a secret in Cloud Secret Manager.
func (a *secretsApp) cmdUpdate(ctx context.Context, project, secretName string) error {
	currentValue, err := a.secretClient.Get(ctx, project, secretName, secret.VersionLatest)
	if err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(a.updateSecret(ctx, project, secretName, currentValue))
}

// updateSecret creates a new version of the given secret, using a ram disk and
// a temporary file containing the current value of the secret and allowing the
// user to hand-edit the file.
func (a *secretsApp) updateSecret(ctx context.Context, project, secretName, currentValue string) error {
	ramdisk, cleanup, err := ramdisk.New(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer cleanup()
	secretFile := filepath.Join(ramdisk, secretName)
	if err := ioutil.WriteFile(secretFile, []byte(currentValue), os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}
	_, _ = fmt.Fprintf(a.stdout, "Wrote secret to %s\n", secretFile)
	_, _ = fmt.Fprintf(a.stdout, "Edit the file and press enter when finished.\n")
	reader := bufio.NewReader(a.stdin)
	if _, err := reader.ReadString('\n'); err != nil {
		return skerr.Wrap(err)
	}
	newValue, err := ioutil.ReadFile(secretFile)
	if err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(a.putSecret(ctx, project, secretName, string(newValue)))
}

// getK8sSecret retrieves the given Kubernetes secret from the given project.
func getK8sSecret(ctx context.Context, project, secretName string) (map[string]string, error) {
	// Find the location of the attach.sh shell script.
	_, filename, _, _ := runtime.Caller(0)
	attachFilename := filepath.Join(filepath.Dir(filename), "../../attach.sh")

	// Retrieve the secret.
	cmd := executil.CommandContext(ctx, attachFilename, project, "kubectl", "get", "secret", secretName, "-o", "jsonpath={.data}")
	outBytes, err := cmd.Output()
	if err != nil {
		return nil, skerr.Wrapf(err, "output: %s", string(outBytes))
	}
	out := strings.TrimSpace(string(outBytes))
	// Take only the last line of output, which contains the secret.
	split := strings.Split(out, "\n")
	out = split[len(split)-1]

	// Decode the secret as JSON.
	var b64 map[string]string
	if err := json.Unmarshal([]byte(out), &b64); err != nil {
		return nil, skerr.Wrapf(err, "failed to decode secret as JSON")
	}

	// The keys are further encoded as base64. Decode.
	rv := make(map[string]string, len(b64))
	for key, b64Value := range b64 {
		value, err := base64.StdEncoding.DecodeString(b64Value)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to decode secret as base 64")
		}
		rv[key] = string(value)
	}
	return rv, nil
}

// putSecret stores the given secret into Cloud Secret Manager.
func (a *secretsApp) putSecret(ctx context.Context, project, name, value string) error {
	version, err := a.secretClient.Update(ctx, project, name, value)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(fmt.Sprintf("Created version %s", version))
	return nil
}

// cmdGrantAccess grants access to a secret in Cloud Secret Manager.
func (a *secretsApp) cmdGrantAccess(ctx context.Context, project string, secretNames, serviceAccounts []string) error {
	for _, secretName := range secretNames {
		for _, serviceAccount := range serviceAccounts {
			if err := a.secretClient.GrantAccess(ctx, project, secretName, serviceAccount); err != nil {
				return skerr.Wrap(err)
			}
		}
	}
	return nil
}

// cmdRevokeAccess revokes access to a secret in Cloud Secret Manager.
func (a *secretsApp) cmdRevokeAccess(ctx context.Context, project string, secretNames, serviceAccounts []string) error {
	for _, secretName := range secretNames {
		for _, serviceAccount := range serviceAccounts {
			if err := a.secretClient.RevokeAccess(ctx, project, secretName, serviceAccount); err != nil {
				return skerr.Wrap(err)
			}
		}
	}
	return nil
}

func main() {
	client, err := secret.NewClient(context.Background())
	if err != nil {
		sklog.Fatal(err)
	}
	app = &secretsApp{
		secretClient: client,
		stdin:        os.Stdin,
		stdout:       os.Stdout,
	}
	defer func() {
		if err := client.Close(); err != nil {
			sklog.Error(err)
		}
	}()
	cliApp.RunAndExitOnError()
}
