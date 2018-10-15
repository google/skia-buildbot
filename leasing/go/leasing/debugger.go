/*
	Used by the Leasing Server to setup debugger on leased Android bots.
*/

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/vcsinfo"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	NUM_COMMITS            = 20
	PUBLIC_BINARIES_BUCKET = "skia-public-binaries"
)

var (
	skiaRepo *gitinfo.GitInfo
	// Mutex that controls access to the above checkout.
	checkoutMtx = sync.Mutex{}

	bucketHandle *storage.BucketHandle
)

func DebuggerInit() error {
	// Checkout Skia repo to query for commit hashes. Used for downloading
	// skiaserve. See skbug.com/7399 for context.
	var err error
	skiaRepo, err = gitinfo.CloneOrUpdate(context.Background(), common.REPO_SKIA, filepath.Join(*workdir, "skia"), false)
	if err != nil {
		return fmt.Errorf("Failed to checkout %s: %s", common.REPO_SKIA, err)
	}

	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_READ_WRITE)
	if err != nil {
		return fmt.Errorf("Problem setting up default token source: %s", err)
	}
	storageClient, err := storage.NewClient(context.Background(), option.WithTokenSource(ts))
	if err != nil {
		return fmt.Errorf("Failed to create a Google Storage API client: %s", err)
	}
	bucketHandle = storageClient.Bucket(PUBLIC_BINARIES_BUCKET)

	return nil
}

func getRevList(n int) ([]*vcsinfo.LongCommit, error) {
	checkoutMtx.Lock()
	defer checkoutMtx.Unlock()
	if err := skiaRepo.Update(context.Background(), true, false); err != nil {
		return nil, fmt.Errorf("Could not update the skia checkout: %s", err)
	}
	commits := []*vcsinfo.LongCommit{}
	for _, c := range skiaRepo.LastNIndex(n) {
		lc, err := skiaRepo.Details(context.Background(), c.Hash, false)
		if err != nil {
			return nil, fmt.Errorf("Could not get commit info for git revision %s: %s", c.Hash, err)
		}
		// Reverse the order so the most recent commit is first
		commits = append([]*vcsinfo.LongCommit{lc}, commits...)
	}
	return commits, nil
}

func GetSkiaServeGSPath(arch string) (string, error) {
	commits, err := getRevList(NUM_COMMITS)
	if err != nil || len(commits) == 0 {
		return "", fmt.Errorf("Could not get commits from skia checkout: %s", err)
	}

	for _, c := range commits {
		query := &storage.Query{
			Prefix: filepath.Join("skiaserve", arch, c.Hash),
		}
		it := bucketHandle.Objects(context.Background(), query)
		for obj, err := it.Next(); err != iterator.Done; obj, err = it.Next() {
			if err != nil {
				// This hash might not have show up yet.
				break
			}
			return fmt.Sprintf("gs://%s/%s", PUBLIC_BINARIES_BUCKET, obj.Name), nil
		}
	}
	return "", fmt.Errorf("Could not find uploaded skiaserve binary for any of the hashes from %v", commits)
}
