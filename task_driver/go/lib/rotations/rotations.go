package rotations

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/rotations"
	"go.skia.org/infra/task_driver/go/td"
)

func get(ctx context.Context, c *http.Client, name, url string) ([]string, error) {
	var reviewers []string
	return reviewers, td.Do(ctx, td.Props(name).Infra(), func(ctx context.Context) error {
		var err error
		reviewers, err = rotations.FromURL(c, url)
		return err
	})
}

func GetCurrentSkiaGardener(ctx context.Context, c *http.Client) ([]string, error) {
	return get(ctx, c, "Get current Skia Gardener", rotations.SkiaGardenerURL)
}

func GetCurrentInfraGardener(ctx context.Context, c *http.Client) ([]string, error) {
	return get(ctx, c, "Get current Infra Gardener", rotations.InfraGardenerURL)
}
