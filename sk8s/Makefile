# Go build command to build static binaries.
KGO := CGO_ENABLED=0 GOOS=linux go install

default:
	go install ./go

release_metadata:
	$(KGO) ./go/metadata
	./images/metadata/release

push_metadata:
	echo "Not implemented yet."

release_rpi_swarming:
	GOOS=linux GOARCH=arm GOARM=7 go build -o out/bot_config -v ./go/bot_config
	./images/rpi-swarming-client/release


push_rpi_swarming:
	echo "Not implemented yet."

