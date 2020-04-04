# Go build command to build static binaries.
default:
	go install ./go/...

release_metadata:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/metadata
	./images/metadata/release

push_metadata:
	pushk metadata

release_rpi_swarming:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -o ./out/bot_config ./go/bot_config 
	./images/rpi-swarming-client/release

push_rpi_swarming:
	pushk rpi-swarming-client