# Go build command to build static binaries.
KGO := CGO_ENABLED=0 GOOS=linux go install

default:
	go install ./go/...

release_metadata:
	$(KGO) ./go/metadata
	./images/metadata/release

push_metadata:
	pushk metadata