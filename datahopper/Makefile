include ../go/depot_tools/depot_tools.mk

datahopper: depot_tools
	go install -v ./go/datahopper

all: datahopper

testgo:
	go test ./...

release:
	./build_release

.PHONY: datahopper release testgo
