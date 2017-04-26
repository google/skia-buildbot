.PHONY: all
all: swarming_logger

.PHONY: swarming_logger
swarming_logger:
	go install -v ./go/swarming_logger

.PHONY: push
push: all
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli swarming-logger skia-swarming-logger
