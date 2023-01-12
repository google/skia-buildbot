package cas

import (
	"context"
	"flag"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/td"
	"golang.org/x/oauth2"
)

type Flags struct {
	Digests  *[]string
	Instance *string
}

type CASDownload struct {
	Path   string
	Digest string
}

// SetupFlags initializes command-line flags used by this package. If a FlagSet
// is not provided, then these become top-level CommandLine flags.
func SetupFlags(fs *flag.FlagSet) *Flags {
	if fs == nil {
		fs = flag.CommandLine
	}
	return &Flags{
		Digests:  common.FSNewMultiStringFlag(fs, "cas", nil, "CAS digests to download, in the form: \"dest/dir:digest/size\""),
		Instance: flag.String("cas-instance", "", "CAS instance to use."),
	}
}

// Download downloads the given CAS digests.
func Download(ctx context.Context, workdir, casInstance string, ts oauth2.TokenSource, dls ...*CASDownload) error {
	return td.Do(ctx, td.Props("Download CAS Inputs").Infra(), func(ctx context.Context) error {
		if len(dls) == 0 {
			return nil
		}
		client, err := rbe.NewClient(ctx, casInstance, ts)
		if err != nil {
			return skerr.Wrap(err)
		}
		for _, dl := range dls {
			dest := filepath.Join(workdir, dl.Path)
			if err := client.Download(ctx, dest, dl.Digest); err != nil {
				return skerr.Wrap(err)
			}
		}
		return nil
	})
}

// DownloadFromFlags downloads the CAS digests requested using the given flags.
func DownloadFromFlags(ctx context.Context, workdir string, ts oauth2.TokenSource, f *Flags) error {
	return td.Do(ctx, td.Props("Download CAS Inputs").Infra(), func(ctx context.Context) error {
		dls, err := GetCASDownloads(f)
		if err != nil {
			return skerr.Wrap(err)
		}
		if len(dls) == 0 {
			return nil
		}
		if *(f.Instance) == "" {
			return skerr.Fmt("--cas-instance is required.")
		}
		client, err := rbe.NewClient(ctx, *f.Instance, ts)
		if err != nil {
			return skerr.Wrap(err)
		}
		for _, dl := range dls {
			dest := filepath.Join(workdir, dl.Path)
			if err := client.Download(ctx, dest, dl.Digest); err != nil {
				return skerr.Wrap(err)
			}
		}
		return nil
	})
}

// GetCASDownloads creates a slice of CASDownload from the Flags.
func GetCASDownloads(f *Flags) ([]*CASDownload, error) {
	if len(*f.Digests) == 0 {
		return nil, nil
	}
	rv := make([]*CASDownload, 0, len(*f.Digests))
	for _, flagStr := range *f.Digests {
		cas := &CASDownload{}
		pathSplit := strings.SplitN(flagStr, ":", 2)
		if len(pathSplit) != 2 {
			return nil, skerr.Fmt("Expected flag in the form \"dest/dir:digest/size\" but got %q", flagStr)
		}
		cas.Path = pathSplit[0]
		cas.Digest = pathSplit[1]
		rv = append(rv, cas)
	}
	return rv, nil
}

// Upload uploads the given paths to CAS and returns a digest.
func Upload(ctx context.Context, workdir, casInstance string, ts oauth2.TokenSource, paths, excludes []string) (string, error) {
	var digest string
	err := td.Do(ctx, td.Props("Upload CAS Outputs").Infra(), func(ctx context.Context) error {
		client, err := rbe.NewClient(ctx, casInstance, ts)
		if err != nil {
			return skerr.Wrap(err)
		}
		digest, err = client.Upload(ctx, workdir, paths, excludes)
		return err
	})
	return digest, err
}
