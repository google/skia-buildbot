datahopper:
	go install -v ./go/datahopper

all: datahopper

testgo:
	go test ./...

release: datahopper
	./build_release "`git log -n1 --format=%s`"

.PHONY: datahopper release testgo
