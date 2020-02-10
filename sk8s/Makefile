# Go build command to build static binaries.
KGO := CGO_ENABLED=0 GOOS=linux go install -a

default:
	go install ./go/...

release_metadata:
	$(KGO) ./go/metadata
	./images/metadata/release

push_metadata:
	pushk metadata

release_rpi_swarming:
	$(KGO) ./go/bot_config
	./images/rpi-swarming-client/release

push_rpi_swarming:
	pushk rpi-swarming-client