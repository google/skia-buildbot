// Backup files to Google Cloud Storage.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

var (
	gceBucket         = flag.String("gce_bucket", "skia-backups", "GCS Bucket backups should be stored in")
	gceFolder         = flag.String("gce_folder", "Swarming", "Folder in the bucket that should hold the backup files")
	localFilePath     = flag.String("local_file_path", "", "Where the file is stored locally on disk. Cannot use with remote_file_path")
	period            = flag.Duration("period", 24*time.Hour, "How often to do the file backup.")
	promPort          = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	remoteCopyCommand = flag.String("remote_copy_command", "scp", "rsync or scp. The router does not have rsync installed.")
	remoteFilePath    = flag.String("remote_file_path", "", "Remote location for a file, to be used by remote_copy_command. E.g. foo@127.0.0.1:/etc/bar.conf Cannot use with local_file_path")
	addHostname       = flag.Bool("add_hostname", false, "If the hostname should be included in the backup file name")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	backupMetric metrics2.Liveness
)

func step(ctx context.Context, storageClient *storage.Client) {
	sklog.Infof("Running backup to %s", *gceFolder)
	if *remoteFilePath != "" {
		// If backing up a remote file, copy it here first, then pretend it is a local file.
		dir, err := ioutil.TempDir("", "backups")
		if err != nil {
			sklog.Fatalf("Could not create temp directory %s: %s", dir, err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				sklog.Errorf("Failed to clean up temp directory %q: %s", dir, err)
			}
		}()
		*localFilePath = path.Join(dir, "placeholder")
		stdOut := bytes.Buffer{}
		stdErr := bytes.Buffer{}
		// This only works if the remote file's host has the source's SSH public key in
		// $HOME/.ssh/authorized_key
		err = exec.Run(ctx, &exec.Command{
			Name:   *remoteCopyCommand,
			Args:   []string{*remoteFilePath, *localFilePath},
			Stdout: &stdOut,
			Stderr: &stdErr,
		})
		sklog.Infof("StdOut of %s command: %s", *remoteCopyCommand, stdOut.String())
		sklog.Infof("StdErr of %s command: %s", *remoteCopyCommand, stdErr.String())
		if err != nil {
			sklog.Fatalf("Could not copy remote file %s: %s", *remoteFilePath, err)
		}
	}

	contents, hash, err := fileutil.ReadAndSha1File(*localFilePath)
	if err != nil {
		sklog.Fatalf("Could not read file %s: %s", *localFilePath, err)
	}

	// We name the file using date and sha1 hash of the file
	day := time.Now().Format("2006-01-02")
	name := fmt.Sprintf("%s/%s-%s.gz", *gceFolder, day, hash)
	if *addHostname {
		if hostname, err := os.Hostname(); err != nil {
			sklog.Warningf("Could not get hostname for file name: %s", err)
		} else {
			name = fmt.Sprintf("%s/%s-%s-%s.gz", *gceFolder, day, hostname, hash)
		}
	}
	w := storageClient.Bucket(*gceBucket).Object(name).NewWriter(ctx)
	defer util.Close(w)

	w.ContentEncoding = "application/gzip"

	gw := gzip.NewWriter(w)

	sklog.Infof("Uploading %s to gs://%s/%s", *localFilePath, *gceBucket, name)

	// This takes a few minutes for a ~1.3 GB image (which gets compressed to about 400MB)
	if i, err := gw.Write([]byte(contents)); err != nil {
		util.Close(gw)
		sklog.Errorf("Problem writing to GCS.  Only wrote %d/%d bytes: %s", i, len(contents), err)
		return
	}

	if err = gw.Close(); err != nil {
		sklog.Errorf("Problem writing to GCS. Error when closing: %s", err)
	} else {
		backupMetric.Reset()
		sklog.Infof("Upload complete")
	}
}

func main() {
	common.InitWithMust(
		"router_backup_ansible",
		common.PrometheusOpt(promPort),
		common.CloudLogging(local, "skia-public"),
	)
	ctx := context.Background()
	if *localFilePath == "" && *remoteFilePath == "" {
		sklog.Fatalf("You must specify a file location")
	}
	if *localFilePath != "" && *remoteFilePath != "" {
		sklog.Fatalf("You must specify a local_file_path OR a remote_file_path, not both")
	}

	tokenSource, err := google.DefaultTokenSource(ctx, storage.ScopeReadWrite)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).With2xxOnly().Client()

	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Could not authenticate to GCS: %s", err)
	}

	backupMetric = metrics2.NewLiveness("skolo_last_backup", nil)

	step(ctx, storageClient)
	for range time.Tick(*period) {
		step(ctx, storageClient)
	}
}
