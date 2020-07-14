build:
	go install ./go/named-fiddles

release: build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/named-fiddles
	./images/named-fiddles/release

push: release
	pushk named-fiddles

testci:
	go test ./go/...