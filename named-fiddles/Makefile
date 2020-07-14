build:
	go install ./go/named-fiddles

release: build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/named-fiddles
	./build_release

push: release
	pushk named-fiddles

testci:
	go test ./go/...