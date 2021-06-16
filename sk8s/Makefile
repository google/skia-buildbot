# Go build command to build static binaries.
default:
	go install ./go/...

release_metadata:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/metadata
	./images/metadata/release

push_metadata: release_metadata
	pushk metadata

release_podwatcher:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/podwatcher
	./images/podwatcher/release

push_podwatcher: release_podwatcher
	pushk podwatcher

release_rpi_swarming:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -o ./out/test_machine_monitor ./go/test_machine_monitor
	./images/rpi-swarming-client/release

push_rpi_swarming: release_rpi_swarming
	pushk rpi-swarming-client