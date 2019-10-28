package drivesource

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v2"
)

type driveSource struct {
	service *drive.Service
}

// New returns a new Source.
func New() (source.Source, error) {
	c, err := google.DefaultClient(context.Background(), drive.DriveMetadataReadonlyScope)
	if err != nil {
		return nil, err
	}
	service, err := drive.New(c)
	if err != nil {
		return nil, err
	}
	return &driveSource{
		service: service,
	}, nil
}

// See source.Source.
func (d *driveSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)

		list, err := d.service.Files.List().
			Context(context.Background()).
			Corpora("drive").
			DriveId(viper.GetString("sources.drive.id")).
			IncludeItemsFromAllDrives(true).
			SupportsAllDrives(true).
			Q(fmt.Sprintf("fullText contains %q", hashtag)).
			Do()
		if err != nil {
			sklog.Errorf("Failed to make Drive request: %s", err)
			return
		}
		for _, entry := range list.Items {
			ts, err := time.Parse(time.RFC3339, entry.ModifiedDate)
			if err != nil {
				sklog.Errorf("Can't parse %q at time: %s", entry.ModifiedDate, err)
				ts = time.Now()
			}
			ret <- source.Artifact{
				Title:        entry.Title,
				URL:          entry.SelfLink,
				LastModified: ts,
			}
		}
	}()

	return ret
}

// See source.Source.
func (d *driveSource) ByUser(string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	close(ret)
	return ret
}
