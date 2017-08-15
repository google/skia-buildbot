include ../go/depot_tools/depot_tools.mk

datahopper: depot_tools
	go install -v ./go/datahopper

all: datahopper

testgo:
	go test ./...

release: datahopper
	./build_release "`git log -n1 --format=%s`"

.PHONY: datahopper release testgo
