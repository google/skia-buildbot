package rotations

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/rotations"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	SHERIFF_URL = "http://tree-status.skia.org/current-sheriff"
	TROOPER_URL = "http://tree-status.skia.org/current-trooper"
)

func get(ctx context.Context, c *http.Client, name, url string) ([]string, error) {
	var reviewers []string
	return reviewers, td.Do(ctx, td.Props(name).Infra(), func(ctx context.Context) error {
		var err error
		reviewers, err = rotations.FromURL(c, url)
		return err
	})
}

func GetCurrentSheriff(ctx context.Context, c *http.Client) ([]string, error) {
	return get(ctx, c, "Get current sheriff", SHERIFF_URL)
}

func GetCurrentTrooper(ctx context.Context, c *http.Client) ([]string, error) {
	return get(ctx, c, "Get current trooper", TROOPER_URL)
}
