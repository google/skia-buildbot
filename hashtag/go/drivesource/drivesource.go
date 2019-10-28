package drivesource

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type driveSource struct {
	service *drive.Service
}

// New returns a new Source.
func New() (source.Source, error) {
	service, err := drive.NewService(context.Background(), option.WithScopes(drive.DriveMetadataReadonlyScope))
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
				URL:          entry.AlternateLink,
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
