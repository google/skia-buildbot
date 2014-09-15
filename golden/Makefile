build:
	go install -v ./go/rbserver

packages:
	go build go/diff/*.go
	go build go/importer/*.go
	go build go/ingester/*.go
	go build go/util/*.go

test:
	go test ./go/web

all: packages build
