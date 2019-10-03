default:
	go install -v ./go/datastore-backup-k

release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/datastore-backup-k

pushk: release
	./build_docker_release
	pushk --cluster=skia-public --message="`git log -n1 --format=%s`" datastore-backup-k
	pushk --cluster=skia-corp   --message="`git log -n1 --format=%s`" datastore-backup-k

indexes:
	gcloud datastore create-indexes ./index.yaml

cleanup-indexes:
	gcloud datastore cleanup-indexes ./index.yaml

.PHONY: default push indexes
