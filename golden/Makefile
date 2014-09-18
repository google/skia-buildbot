build:
	go install -v ./go/rbserver
	go install -v ./go/goldingest

packages:
	go build go/diff/
	go build go/importer/
	go build go/ingester/
	go build go/util/

test:
	go test ./go/types ./go/goldingester

all: packages build
