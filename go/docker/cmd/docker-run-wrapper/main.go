package main

/*
docker-run-wrapper is a wrapper around "docker run" which provides things like
authentication to replicate the setup in GKE or GCB.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
)

var (
	projectMetadata = map[string]string{
		"project-id": "skia-infra-public",
	}
	instanceMetadata = map[string]string{}
)

type Config struct {
	Image   string
	Port    string
	WorkDir string
	HomeDir string
	User    string
	DinD    bool
	Volumes []string
	Args    []string
}

func main() {
	var cfg Config
	app := &cli.App{
		Name:        "docker-run-wrapper",
		Description: "docker-run-wrapper is a wrapper around\"docker run\" which provides authentication to replicate the setup in GKE or GCB.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "image",
				Usage:       "The docker image to run.",
				Required:    true,
				Destination: &cfg.Image,
			},
			&cli.StringFlag{
				Name:        "port",
				Usage:       "Run the metadata server on this port.",
				Value:       "8000",
				Required:    false,
				Destination: &cfg.Port,
			},
			&cli.StringFlag{
				Name:        "workdir",
				Usage:       "Current working directory to use inside the container. May be specified with a colon to map a host directory into the container, eg \"--workdir=/container/dir\" or \"--workdir=/host/dir:/container/dir\".",
				Value:       "/workspace",
				Required:    false,
				Destination: &cfg.WorkDir,
			},
			&cli.StringFlag{
				Name:        "home",
				Usage:       "Home directory inside the container.",
				Value:       "/home/builder",
				Required:    false,
				Destination: &cfg.HomeDir,
			},
			&cli.StringFlag{
				Name:        "user",
				Usage:       "User to run as. If not specified, runs as the default user of the image. Follows the same semantics as the \"--user\" flag to \"docker run\", ie. it can be a user name or a user ID with optional group, eg. \"--user=myuser\", \"--user=1000\", \"--user=1000:1000\". Setting the special value \"@me\" will use the current host user ID and group ID.",
				Value:       "",
				Required:    false,
				Destination: &cfg.User,
			},
			&cli.BoolFlag{
				Name:        "dind",
				Usage:       "Enables settings required for docker-in-docker, including privileged container and mounting /var/run/docker.sock into the container.",
				Value:       false,
				Required:    false,
				Destination: &cfg.DinD,
			},
			&cli.MultiStringFlag{
				Target: &cli.StringSliceFlag{
					Name:    "volume",
					Usage:   "Volumes to mount. Directly passed through to \"docker run\".",
					Aliases: []string{"v"},
				},
				Value:       cfg.Volumes,
				Destination: &cfg.Volumes,
			},
		},
		Action: func(ctx *cli.Context) error {
			cfg.Args = ctx.Args().Slice()
			if err := dockerRun(ctx.Context, cfg); err != nil {
				return cli.Exit(err, 1)
			}
			return nil
		},
	}
	sklog.Fatal(app.Run(os.Args))
}

func dockerRun(ctx context.Context, cfg Config) error {
	// Start the metadata server.
	host := "localhost"
	if err := runMetadataServer(cfg.Port); err != nil {
		return skerr.Wrapf(err, "failed to start metadata server")
	}
	sklog.Infof("Server running on :%s", cfg.Port)

	// Handle --user.
	user, err := user.Current()
	if err != nil {
		return skerr.Wrapf(err, "failed to find home directory")
	}
	userFlag := cfg.User
	userEnvVar := ""
	if userFlag == "@me" {
		userFlag = fmt.Sprintf("%s:%s", user.Uid, user.Gid)
		userEnvVar = fmt.Sprintf("USER=%s", user.Username)
	}

	// Create a temporary directory.
	wd, err := os.MkdirTemp("", "docker-run-wrapper-")
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.RemoveAll(wd)
	if err := os.Chmod(wd, 0755); err != nil {
		return skerr.Wrap(err)
	}

	// Find application default credentials.
	const adcEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"
	adcSrcPath := os.Getenv(adcEnvVar)
	if adcSrcPath == "" {
		adcSrcPath = filepath.Join(user.HomeDir, ".config", "gcloud", "application_default_credentials.json")
	}
	if _, err = os.Stat(adcSrcPath); os.IsNotExist(err) {
		return skerr.Fmt("application default credentials file %s does not exist", adcSrcPath)
	} else if err != nil {
		return skerr.Wrap(err)
	}
	// Write to a new temporary file so that the container can have read perms.
	adcHostPath := filepath.Join(wd, "application_default_credentials.json")
	if err := util.CopyFile(adcSrcPath, adcHostPath); err != nil {
		return skerr.Wrap(err)
	}
	if err := os.Chmod(adcHostPath, 0644); err != nil {
		return skerr.Wrap(err)
	}
	adcContainerPath := filepath.Join(cfg.HomeDir, "application_default_credentials.json")

	// Map the workdir as requested.
	hostWorkDir := ""
	containerWorkDir := ""
	if split := strings.SplitN(cfg.WorkDir, ":", 2); len(split) == 1 {
		hostWorkDir = filepath.Join(wd, "workspace")
		if err := os.Mkdir(hostWorkDir, 0777); err != nil {
			return skerr.Wrapf(err, "failed to create workdir")
		}
		if err := os.Chmod(hostWorkDir, 0777); err != nil {
			return skerr.Wrap(err)
		}
		containerWorkDir = split[0]
	} else if len(split) == 2 {
		hostWorkDir = split[0]
		containerWorkDir = split[1]
	}

	// Run Docker auth.
	hostDockerConfigFilePath, err := docker.AutoUpdateConfigFileAuth(ctx)
	if err != nil {
		return skerr.Wrapf(err, "failed to run Docker auth")
	}
	containerDockerConfigFilePath := cfg.HomeDir + "/.docker/config.json"

	// Run the command.
	cmd := []string{
		"docker", "run",
		"--network", "host",
		"--env", fmt.Sprintf("HOME=%s", cfg.HomeDir),
		"--env", fmt.Sprintf("GCE_METADATA_HOST=%s:%s", host, cfg.Port),
		"--env", fmt.Sprintf("%s=%s", adcEnvVar, adcContainerPath),
		"--volume", fmt.Sprintf("%s:%s", adcHostPath, adcContainerPath),
		"--volume", fmt.Sprintf("%s:%s", hostDockerConfigFilePath, containerDockerConfigFilePath),
	}
	if containerWorkDir != "" && hostWorkDir != "" {
		cmd = append(cmd, "--workdir", containerWorkDir)
		cmd = append(cmd, "--volume", fmt.Sprintf("%s:%s", hostWorkDir, containerWorkDir))
	}
	if userFlag != "" {
		cmd = append(cmd, "--user", userFlag)
	}
	if userEnvVar != "" {
		cmd = append(cmd, "--env", userEnvVar)
	}
	if cfg.DinD {
		cmd = append(cmd, "--privileged")
		cmd = append(cmd, "--volume", "/var/run/docker.sock:/var/run/docker.sock")
	}
	for _, volume := range cfg.Volumes {
		cmd = append(cmd, "--volume", volume)
	}
	cmd = append(cmd, cfg.Image)
	cmd = append(cmd, cfg.Args...)
	sklog.Info(strings.Join(cmd, " "))
	_, err = exec.RunCommand(ctx, &exec.Command{
		Name:   cmd[0],
		Args:   cmd[1:],
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	return skerr.Wrap(err)
}

// runMetadataServer starts up a metadata server.
func runMetadataServer(port string) error {
	r := chi.NewRouter()
	r.HandleFunc("/computeMetadata/v1/instance/service-accounts/{serviceAccount}/token", tokenHandler)
	r.HandleFunc("/computeMetadata/v1/{category}/*", func(w http.ResponseWriter, r *http.Request) {
		key := strings.Join(strings.Split(r.URL.Path, "/")[4:], "/")
		var value string
		var ok bool
		switch chi.URLParam(r, "category") {
		case "project":
			value, ok = projectMetadata[key]
		case "instance":
			value, ok = instanceMetadata[key]
		default:
			http.Error(w, "unknown metadata category", http.StatusNotFound)
			return
		}
		if !ok {
			http.Error(w, "unknown metadata key", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(value))
	})

	h := httputils.LoggingRequestResponse(r)
	go func() {
		sklog.Fatal(http.ListenAndServe(":"+port, h))
	}()
	return nil
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
	scopes := r.URL.Query()["scopes"]
	ts, err := google.DefaultTokenSource(r.Context(), scopes...)
	if err != nil {
		httputils.ReportError(w, err, "failed to get TokenSource", http.StatusInternalServerError)
		return
	}
	tok, err := ts.Token()
	// TODO(borenet): This is required, but I'm not sure why it's not already
	// filled in, or derived when needed.
	tok.ExpiresIn = int64(time.Until(tok.Expiry).Seconds())
	if err != nil {
		httputils.ReportError(w, err, "failed to get Token", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(tok); err != nil {
		httputils.ReportError(w, err, "failed to get encode response", http.StatusInternalServerError)
		return
	}
}
