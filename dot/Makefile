build:
	go install ./go/dotserver

release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/dotserver
	./build_release

push: release
	pushk dotserver

test:
	go test ./go/...