build:
	go install -v ./go/rbserver

packages:
	go build go/diff/
	go build go/importer/
	go build go/ingester/
	go build go/util/

test:
	go test ./go/types

all: packages build
