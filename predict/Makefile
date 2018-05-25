.PHONY: build release push
build:
	go install  ./go/...

release:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/predict
	./build_release

push:
	pushk --apply predict
