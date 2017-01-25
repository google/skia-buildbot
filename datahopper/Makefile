datahopper:
	go install -v ./go/datahopper

all: datahopper

testgo:
	go test ./...

release:
	./build_release

.PHONY: datahopper release testgo
