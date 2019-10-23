package drivesource

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

type driveSource struct {
	ds *drive.FilesService
}

// New returns a new Source.
func New() (source.Source, error) {
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, drive.DriveReadonlyScope)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	driveService, err := drive.New(client)
	ret := &driveSource{
		ds: driveService.Files,
	}
	return ret, nil

}

func (g *driveSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		// list, err := g.ds.List().Corpora("user").IncludeItemsFromAllDrives(true).SupportsAllDrives(true).Q(fmt.Sprintf("fullText contains '#%s'", hashtag)).Do()
		list, err := g.ds.List().Corpora("user").IncludeItemsFromAllDrives(true).SupportsAllDrives(true).Do()
		if err != nil {
			sklog.Errorf("Failed to run Drive search: %s", err)
			return
		}
		for _, file := range list.Files {
			ts, err := time.Parse(time.RFC3339, file.ModifiedTime)
			if err != nil {
				sklog.Errorf("Failed to parse drive file time: %s", err)
				ts = time.Time{}
			}

			ret <- source.Artifact{
				Title:        file.Description,
				URL:          file.WebContentLink,
				LastModified: ts,
				Kind:         source.Drive,
			}
		}
	}()

	return ret
}

func (g *driveSource) ByUser(string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	close(ret)
	return ret
}
