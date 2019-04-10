datahopper:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/datahopper

all: datahopper

testgo:
	go test ./...

release: datahopper
	./build_docker_release

.PHONY: datahopper release testgo
