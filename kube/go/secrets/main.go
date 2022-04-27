package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var (
	// CLI options.
	app = &cli.App{
		Name:        "secrets",
		Description: "Provides tools for working with secrets.",
		Commands: []*cli.Command{
			{
				Name:        "describe",
				Description: "Prints information about a secret in Kubernetes.",
				Usage:       "describe <src project> <src secret name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 2 {
						return skerr.Fmt("Expected 2 positional arguments.")
					}
					return cmdDescribe(ctx.Context, args[0], args[1])
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
					return cmdMigrate(ctx.Context, args[0], args[1], args[2], args[3], args[4])
				},
			},
		},
		Usage: "secrets <subcommand>",
	}
)

// cmdDescribe prints information about a secret in Kubernetes.
func cmdDescribe(ctx context.Context, srcProject, srcSecretName string) error {
	secretMap, err := getSecret(ctx, srcProject, srcSecretName)
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
func cmdMigrate(ctx context.Context, srcProject, srcSecretName, srcSecretPath, dstProject, dstSecretName string) error {
	secretMap, err := getSecret(ctx, srcProject, srcSecretName)
	if err != nil {
		return skerr.Wrap(err)
	}
	value, ok := secretMap[srcSecretPath]
	if !ok {
		return skerr.Fmt("secret %q has no value at path %q", srcSecretName, srcSecretPath)
	}
	return skerr.Wrap(putSecret(ctx, dstProject, dstSecretName, value))
}

// getSecret retrieves the given Kubernetes secret from the given project.
func getSecret(ctx context.Context, project, secretName string) (map[string]string, error) {
	// Find the location of the attach.sh shell script.
	_, filename, _, _ := runtime.Caller(0)
	attachFilename := filepath.Join(filepath.Dir(filename), "../../attach.sh")

	// Retrieve the secret.
	cmd := []string{attachFilename, project, "kubectl", "get", "secret", secretName, "-o", "jsonpath={.data}"}
	out, err := exec.RunCwd(ctx, ".", cmd...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
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
func putSecret(ctx context.Context, project, name, value string) error {
	// Store the secret.
	// Note: because this tool is designed for first-time migration of secrets,
	// we're unconditionally creating the secret first.  Creation of a secret
	// which already exists should fail, which is what we want for this use
	// case.
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer func() {
		if err := secretClient.Close(); err != nil {
			sklog.Error(err)
		}
	}()

	if err := secretClient.Create(ctx, project, name); err != nil {
		return skerr.Wrap(err)
	}
	version, err := secretClient.Update(ctx, project, name, value)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(fmt.Sprintf("Created version %s", version))
	return nil
}

func main() {
	app.RunAndExitOnError()
}
