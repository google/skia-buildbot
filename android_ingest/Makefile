.PHONY: build
build:
	go install -v ./go/...

.PHONY: test
test:
	go test ./go/...

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/androidingest
	./images/androidingest/release

.PHONY: push
push: release
	pushk androidingest