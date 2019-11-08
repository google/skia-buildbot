# Go build command to build static binaries.
KGO := CGO_ENABLED=0 GOOS=linux go install

default:
	go install ./go

release_metadata:
	$(KGO) ./go/metadata
	./images/metadata/release

push_metadata:
	echo "Not implemented yet."


release_rpi_client:
	./images/rpi_client/release