package drivesource

import (
	"context"
	"fmt"
	"strings"
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

// toString converts a source.Query into a search string for the Drive API.
func (d *driveSource) toString(q source.Query) string {
	ret := []string{}
	// Documents stored in a Teams folder have their owners changed to the group
	// that owns the team drive, so we always just do a fullText search.
	ret = append(ret, fmt.Sprintf("fullText contains '%s'", q.Value))
	/*
		TODO(jcgregorio) This code currently makes the query invalid. I have no idea why.

		if !q.Begin.IsZero() {
			ret = append(ret, fmt.Sprintf("modifiedTime>'%s'", q.Begin.Format("2006-01-02")))
		}
		if !q.End.IsZero() {
			ret = append(ret, fmt.Sprintf("modifiedTime<'%s'", q.End.Format("2006-01-02")))
		}
	*/
	sklog.Infof("Drive: %q", strings.Join(ret, " AND "))
	return strings.Join(ret, " AND ")
}

// See source.Source.
func (d *driveSource) Search(ctx context.Context, q source.Query) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)

		list, err := d.service.Files.List().
			Context(ctx).
			Corpora("drive").
			DriveId(viper.GetString("sources.drive.id")).
			IncludeItemsFromAllDrives(true).
			SupportsAllDrives(true).
			Q(d.toString(q)).
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
