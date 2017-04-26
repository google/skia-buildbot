all: swarming_logger

swarming_logger:
	go install -v ./go/swarming_logger

release: swarming_logger
	./build_release "$(MESSAGE)"
