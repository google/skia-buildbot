package internal

import (
	"context"
	"fmt"

	"go.skia.org/infra/pinpoint/go/backends"
)

type GerritServiceActivity struct {
	insecure_conn bool
}

func (gsa *GerritServiceActivity) GetCommitRevision(ctx context.Context, commitPostion int64) (string, error) {
	client, err := backends.NewCrrevClient(ctx)
	if err != nil {
		return "", err
	}
	resp, err := client.GetCommitInfo(ctx, fmt.Sprint(commitPostion))
	if err != nil {
		return "", err
	}
	return resp.GitHash, nil
}
