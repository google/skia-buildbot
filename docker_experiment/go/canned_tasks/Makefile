.PHONY: build
build:
	for step in "$$(find -maxdepth 1 -type d | cut -s -d / -f 2)"; do \
		echo $${step}; \
		CGO_ENABLED=0 GOOS=linux go install -a ./$${step}; \
	done

.PHONY: release
release: build
	./build_release

default: build
