package bbstate

import "go.skia.org/infra/golden/go/tryjobstore"

type bbServiceV2 struct{}

func newBuildBucketV2() (BuildBucketSvc, error) {
	return nil, nil
}

func (b *bbServiceV2) Get(buildBucketID int64) (*tryjobstore.Tryjob, error) {
	return nil, nil
}
